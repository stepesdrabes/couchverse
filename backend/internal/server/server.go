package server

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"io"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"couchverse/internal/config"
	"couchverse/internal/feature/analytics"
	"couchverse/internal/feature/artwork"
	"couchverse/internal/feature/auth"
	"couchverse/internal/feature/catalog"
	"couchverse/internal/feature/couch"
	"couchverse/internal/feature/downloads"
	"couchverse/internal/feature/jobs"
	"couchverse/internal/feature/library"
	"couchverse/internal/feature/metadata"
	"couchverse/internal/feature/playback"
	"couchverse/internal/feature/ranks"
	"couchverse/internal/feature/subtitles"
	"couchverse/internal/feature/system"
	"couchverse/internal/grant"
	"couchverse/internal/httpx"
	"couchverse/internal/settings"
	"couchverse/web"
)

// Deps are the stores and services the HTTP layer composes, constructed in
// cmd/couchverse.
type Deps struct {
	Config    config.Config
	Pool      *pgxpool.Pool
	Settings  *settings.Store
	Auth      *auth.Store
	Catalog   *catalog.Store
	Jobs      *jobs.Store
	Library   *library.Store
	System    *system.Store
	Uploads   *library.Manager
	Artwork   *artwork.Service
	Subtitles *subtitles.Service
	Transcode *playback.JobHandler
	Sessions  *playback.SessionManager
	Stream    *playback.Stream
	Analytics *analytics.Store
	Ranks     *ranks.Store
	Couch     *couch.Hub
	Grants    *grant.Signer
	Downloads *downloads.Store
}

type Server struct {
	Deps
}

func New(d Deps) *Server {
	return &Server{Deps: d}
}

// OpenAPI returns the API description without a database or running server:
// operations only reference their handlers, so empty dependencies suffice.
func OpenAPI() *huma.OpenAPI {
	s := New(Deps{
		Artwork:   &artwork.Service{},
		Subtitles: &subtitles.Service{},
		Uploads:   &library.Manager{},
	})
	return s.register(chi.NewRouter()).OpenAPI()
}

func (s *Server) Handler() http.Handler {
	sessions := auth.NewMiddleware(s.Auth, s.Config.CookieSecure)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(clientIP(s.Config.TrustedProxies))
	r.Use(requestLogger)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", s.handleHealthz)
	r.Get("/favicon.svg", system.NewTheme(s.Settings).Favicon)

	r.Route("/api/v1", func(v1 chi.Router) {
		v1.Use(auth.CSRFOrigin)
		v1.Use(sessions.Load)
		v1.NotFound(func(w http.ResponseWriter, _ *http.Request) {
			httpx.NotFound(w)
		})
		s.register(v1)
	})

	r.NotFound(spaHandler())
	return r
}

// register builds the API on v1 and lets every feature add its operations to the
// group matching who may call them.
func (s *Server) register(v1 chi.Router) huma.API {
	api := httpx.NewAPI(v1)
	api.OpenAPI().Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		"cookieSession": {Type: "apiKey", In: "cookie", Name: auth.SessionCookie, Description: "Browsers."},
		"bearerDevice":  {Type: "http", Scheme: "bearer", Description: "Native clients: a device token from signInDevice, pollPairing or connectDevice."},
	}
	signedIn := func(op *huma.Operation) {
		op.Security = []map[string][]string{{"cookieSession": {}}, {"bearerDevice": {}}}
	}

	user := huma.NewGroup(api)
	user.UseMiddleware(httpx.Guard(api, auth.SignedIn))
	user.UseSimpleModifier(signedIn)

	admin := huma.NewGroup(api, "/admin")
	admin.UseMiddleware(httpx.Guard(api, auth.Admin))
	admin.UseSimpleModifier(signedIn)

	media := huma.NewGroup(api, "/media/{grant}")
	media.UseMiddleware(s.mediaGrant(api))
	media.UseSimpleModifier(func(op *huma.Operation) {
		op.Parameters = append([]*huma.Param{{
			Name:        "grant",
			In:          "path",
			Required:    true,
			Description: "The media grant from the playback payload; it expires, so fetch the payload again on grant_expired.",
			Schema:      &huma.Schema{Type: huma.TypeString, Pattern: "^[A-Za-z0-9_-]+$"},
		}}, op.Parameters...)
	})

	artworkGroup := huma.NewGroup(api)
	artworkGroup.UseMiddleware(s.artworkAccess(api))

	rt := httpx.Routes{Public: api, User: user, Admin: admin, Media: media, Artwork: artworkGroup}

	auth.NewModule(s.Auth, s.Config, s.Artwork, s.Grants).Register(rt)
	system.NewModule(s.System, s.Settings, s.Jobs, s.Config.DataDir, s.Couch, s.Sessions).Register(rt)
	catalog.NewModule(s.Catalog, s.Settings, s.Artwork, s.Jobs, s.Analytics).Register(rt)
	playback.NewModule(s.Stream, playback.NewAdminTranscode(s.Library, s.Settings, s.Jobs, s.Transcode, s.Config.FFmpegPath)).Register(rt)
	couch.NewModule(s.Couch, s.Settings).Register(rt)
	ranks.NewModule(s.Ranks, s.Settings).Register(rt)
	artwork.NewHandlers(s.Artwork).Register(rt)
	subtitles.NewSubtitles(s.Subtitles.Subs, s.Library, s.Subtitles).Register(rt)
	library.NewModule(s.Library, s.Uploads).Register(rt)
	metadata.NewAdminMetadata(s.Catalog, s.Settings, s.Jobs).Register(rt)
	jobs.NewAdminJobs(s.Jobs).Register(rt)
	analytics.NewModule(s.Analytics).Register(rt)
	downloads.NewHandlers(s.Downloads, s.Catalog, s.Library, s.Subtitles.Subs, s.Jobs, s.Settings, s.Grants, s.Config.FFmpegPath, s.Config.DataDir).Register(rt)
	return api
}

// mediaGrant admits a request whose path carries a valid media grant and puts the
// grant on the context; the media file it names is the only one the request can
// reach. A couch follower's grant also needs the follower to still be on a couch
// playing that file; the check lives here because playback must not import
// couch.
func (s *Server) mediaGrant(api huma.API) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		g, err := s.Grants.Verify(ctx.Param("grant"))
		switch {
		case errors.Is(err, grant.ErrExpired):
			httpx.Reject(api, ctx, httpx.Fail(http.StatusForbidden, "grant_expired", "the media grant expired; fetch the playback payload again"))
		case err != nil || g.Scope != grant.Media:
			httpx.Reject(api, ctx, httpx.Fail(http.StatusForbidden, "invalid_grant", "the media grant is not valid"))
		case g.Couch != uuid.Nil && (s.Couch == nil || !s.Couch.AllowsMedia(ctx.Context(), g.Couch.String(), g.Resource.String())):
			httpx.Reject(api, ctx, httpx.Fail(http.StatusForbidden, "grant_revoked", "the couch session moved on; fetch the follower payload again"))
		default:
			next(huma.WithContext(ctx, grant.WithGrant(ctx.Context(), g)))
		}
	}
}

// artworkAccess admits a signed-in request or one carrying a valid artwork grant
// in ?g=, for images fetched outside the session.
func (s *Server) artworkAccess(api huma.API) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		if auth.UserFrom(ctx.Context()) != nil {
			next(ctx)
			return
		}
		if g, err := s.Grants.Verify(ctx.Query("g")); err == nil && g.Scope == grant.Artwork {
			next(ctx)
			return
		}
		httpx.Reject(api, ctx, httpx.Fail(http.StatusUnauthorized, "unauthorized", "authentication required"))
	}
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.Pool.Ping(ctx); err != nil {
		httpx.Error(w, http.StatusServiceUnavailable, "db_unreachable", "database unreachable")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// spaHandler serves the embedded web build, falling back to index.html
// so client-side routes resolve on deep links and reloads.
func spaHandler() http.HandlerFunc {
	dist, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		panic(err)
	}
	return serveSPA(dist)
}

// encodings are the precompressed siblings the web build writes, best first.
var encodings = []struct{ name, suffix string }{{"br", ".br"}, {"gzip", ".gz"}}

func serveSPA(dist fs.FS) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" || path == "index.html" || !exists(dist, path) {
			// client-side routes resolve in the app; the shell itself is never cached
			w.Header().Set("Cache-Control", "no-cache")
			path = "index.html"
		} else if strings.HasPrefix(path, "_app/immutable/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		serveFile(w, r, dist, path)
	}
}

// serveFile writes a file, or its precompressed sibling when the client accepts that
// encoding; the content type follows the original name either way.
func serveFile(w http.ResponseWriter, r *http.Request, dist fs.FS, path string) {
	w.Header().Add("Vary", "Accept-Encoding")
	name := path
	for _, enc := range encodings {
		if accepts(r.Header.Get("Accept-Encoding"), enc.name) && exists(dist, path+enc.suffix) {
			w.Header().Set("Content-Encoding", enc.name)
			name = path + enc.suffix
			break
		}
	}
	f, err := dist.Open(name)
	if err != nil {
		http.Error(w, "web client not built - run `make build`", http.StatusInternalServerError)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		http.Error(w, "web client not built - run `make build`", http.StatusInternalServerError)
		return
	}
	if ct := mime.TypeByExtension(filepath.Ext(path)); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	content, ok := f.(io.ReadSeeker)
	if !ok {
		http.Error(w, "unseekable asset", http.StatusInternalServerError)
		return
	}
	http.ServeContent(w, r, path, info.ModTime(), content)
}

func exists(dist fs.FS, path string) bool {
	info, err := fs.Stat(dist, path)
	return err == nil && !info.IsDir()
}

// accepts reports whether an Accept-Encoding header allows enc (a q of 0 refuses it).
func accepts(header, enc string) bool {
	for _, part := range strings.Split(header, ",") {
		name, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		if !strings.EqualFold(strings.TrimSpace(name), enc) {
			continue
		}
		q := strings.ReplaceAll(strings.TrimSpace(params), " ", "")
		return q != "q=0" && q != "q=0.0" && q != "q=0.00" && q != "q=0.000"
	}
	return false
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		slog.Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"bytes", ww.BytesWritten(),
			"dur", time.Since(start).Round(time.Millisecond).String(),
		)
	})
}
