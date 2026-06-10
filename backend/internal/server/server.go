package server

import (
	"context"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"couchverse/internal/api"
	"couchverse/internal/config"
	"couchverse/internal/feature/artwork"
	"couchverse/internal/feature/auth"
	"couchverse/internal/feature/catalog"
	"couchverse/internal/feature/jobs"
	"couchverse/internal/feature/library"
	"couchverse/internal/feature/metadata"
	"couchverse/internal/feature/music"
	"couchverse/internal/feature/subtitles"
	"couchverse/internal/flags"
	"couchverse/internal/httpx"
	"couchverse/internal/settings"
	"couchverse/internal/store"
	"couchverse/internal/transcode"
	"couchverse/web"
)

type Server struct {
	cfg       config.Config
	store     *store.Store
	settings  *settings.Store
	auth      *auth.Store
	catalog   *catalog.Store
	jobs      *jobs.Store
	music     *music.Store
	library   *library.Store
	uploads   *library.Manager
	artwork   *artwork.Service
	subtitles *subtitles.Service
	transcode *transcode.JobHandler
	sessions  *transcode.SessionManager
}

func New(cfg config.Config, st *store.Store, set *settings.Store, au *auth.Store, cat *catalog.Store, jb *jobs.Store, mus *music.Store, lib *library.Store, uploads *library.Manager, art *artwork.Service, subs *subtitles.Service, tc *transcode.JobHandler, sessions *transcode.SessionManager) *Server {
	return &Server{cfg: cfg, store: st, settings: set, auth: au, catalog: cat, jobs: jb, music: mus, library: lib, uploads: uploads, artwork: art, subtitles: subs, transcode: tc, sessions: sessions}
}

func (s *Server) Handler() http.Handler {
	sessions := auth.NewMiddleware(s.auth)
	authModule := auth.NewModule(s.auth, s.cfg, s.artwork)
	adminSettings := api.NewAdminSettings(s.settings)
	libraryModule := library.NewModule(s.library, s.jobs, s.uploads)
	adminJobs := jobs.NewAdminJobs(s.jobs)
	catalogModule := catalog.NewModule(s.catalog, s.settings, s.artwork, s.music, s.jobs)
	stream := api.NewStream(s.subtitles.Subs, s.catalog, s.library, s.settings, s.jobs, s.cfg.DataDir, s.sessions, s.cfg.FFmpegPath)
	transcodeAPI := api.NewAdminTranscode(s.library, s.settings, s.jobs, s.transcode, s.cfg.FFmpegPath)
	musicModule := music.NewModule(s.music, s.settings, s.artwork)
	adminStorage := api.NewAdminStorage(s.store, s.jobs, s.cfg.DataDir)
	sysStats := api.NewSysStats()
	theme := api.NewTheme(s.settings)
	artworkAPI := artwork.NewHandlers(s.artwork)
	subtitlesAPI := subtitles.NewSubtitles(s.subtitles.Subs, s.library, s.subtitles)
	metadataAPI := metadata.NewAdminMetadata(s.catalog, s.settings, s.jobs)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(requestLogger)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", s.handleHealthz)

	r.Route("/api/v1", func(v1 chi.Router) {
		v1.Use(auth.CSRFOrigin)
		v1.Use(sessions.Load)
		v1.NotFound(func(w http.ResponseWriter, _ *http.Request) {
			httpx.NotFound(w)
		})

		authModule.MountPublic(v1)
		v1.Get("/theme", theme.Get) // public: accent applies on the login screen too

		// authenticated routes
		v1.Group(func(p chi.Router) {
			p.Use(auth.RequireAuth)
			authModule.MountUser(p)
			catalogModule.MountUser(p)

			p.Get("/stream/{id}", stream.Serve)
			p.Get("/stream/{id}/hls/master.m3u8", stream.HLSMaster)
			p.Get("/stream/{id}/hls/{variant}/{file}", stream.HLSFile)
			p.Post("/stream/{id}/sessions", stream.CreateSession)
			p.Get("/stream/sessions/{sid}/{file}", stream.SessionFile)
			p.Post("/stream/sessions/{sid}/keepalive", stream.SessionKeepalive)
			p.Get("/playback/{kind}/{id}", stream.Playback)
			artworkAPI.MountUser(p)
			subtitlesAPI.MountUser(p)

			p.Get("/features", func(w http.ResponseWriter, r *http.Request) {
				httpx.JSON(w, http.StatusOK, flags.Load(r.Context(), s.settings))
			})

			// music routes (incl. track playlists) gate themselves on the feature toggle
			musicModule.MountUser(p)

		})

		// admin routes
		v1.Route("/admin", func(adm chi.Router) {
			adm.Use(auth.RequireAdmin)

			catalogModule.MountAdmin(adm)

			musicModule.MountAdmin(adm)

			authModule.MountAdmin(adm)

			adm.Get("/settings", adminSettings.Get)
			adm.Put("/settings", adminSettings.Put)

			libraryModule.MountAdmin(adm)

			adminJobs.MountAdmin(adm)

			metadataAPI.MountAdmin(adm)

			artworkAPI.MountAdmin(adm)

			subtitlesAPI.MountAdmin(adm)

			adm.Get("/storage", adminStorage.Get)
			adm.Get("/overview", adminStorage.Overview)
			adm.Get("/system", sysStats.Get)
			adm.Get("/home-rows", adminStorage.HomeRowsGet)
			adm.Put("/home-rows", adminStorage.HomeRowsPut)

			adm.Get("/transcode/info", transcodeAPI.Info)
			adm.Get("/transcode/active", transcodeAPI.Active)
			adm.Post("/media-files/{id}/transcode", transcodeAPI.Enqueue)
			adm.Get("/media-files/{id}/variants", transcodeAPI.ListVariants)
			adm.Delete("/transcode-variants/{id}", transcodeAPI.DeleteVariant)
		})
	})

	r.NotFound(spaHandler())
	return r
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.store.Ping(ctx); err != nil {
		httpx.Error(w, http.StatusServiceUnavailable, "db_unreachable", "database unreachable")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// spaHandler serves the embedded frontend build, falling back to index.html
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
			http.Error(w, "frontend not built — run `make build`", http.StatusInternalServerError)
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
