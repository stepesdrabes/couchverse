package server

import (
	"context"
	"io/fs"
	"log/slog"
	"net/http"
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
	"couchverse/internal/feature/jobs"
	"couchverse/internal/feature/library"
	"couchverse/internal/feature/metadata"
	"couchverse/internal/feature/playback"
	"couchverse/internal/feature/ranks"
	"couchverse/internal/feature/subtitles"
	"couchverse/internal/feature/system"
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
	sessions := auth.NewMiddleware(s.Auth)

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
		"cookieSession": {Type: "apiKey", In: "cookie", Name: auth.SessionCookie},
	}
	signedIn := func(op *huma.Operation) {
		op.Security = []map[string][]string{{"cookieSession": {}}}
	}

	user := huma.NewGroup(api)
	user.UseMiddleware(httpx.Guard(api, auth.SignedIn))
	user.UseSimpleModifier(signedIn)

	admin := huma.NewGroup(api, "/admin")
	admin.UseMiddleware(httpx.Guard(api, auth.Admin))
	admin.UseSimpleModifier(signedIn)

	stream := huma.NewGroup(api)
	stream.UseMiddleware(s.authOrCouch(api))
	stream.UseSimpleModifier(signedIn)

	rt := httpx.Routes{Public: api, User: user, Admin: admin, Stream: stream}

	auth.NewModule(s.Auth, s.Config, s.Artwork).Register(rt)
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
	return api
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
	fileServer := http.FileServer(http.FS(dist))

	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path != "" && path != "index.html" {
			if f, err := dist.Open(path); err == nil {
				f.Close()
				if strings.HasPrefix(path, "_app/immutable/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				fileServer.ServeHTTP(w, r)
				return
			}
		}

		index, err := fs.ReadFile(dist, "index.html")
		if err != nil {
			http.Error(w, "web client not built - run `make build`", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Write(index)
	}
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
