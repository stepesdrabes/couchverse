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
	"couchverse/internal/feature/jobs"
	"couchverse/internal/feature/music"
	"couchverse/internal/flags"
	"couchverse/internal/httpx"
	"couchverse/internal/settings"
	"couchverse/internal/store"
	"couchverse/internal/subtitles"
	"couchverse/internal/transcode"
	"couchverse/internal/upload"
	"couchverse/web"
)

type Server struct {
	cfg       config.Config
	store     *store.Store
	settings  *settings.Store
	auth      *auth.Store
	jobs      *jobs.Store
	music     *music.Store
	uploads   *upload.Manager
	artwork   *artwork.Service
	subtitles *subtitles.Service
	transcode *transcode.JobHandler
	sessions  *transcode.SessionManager
}

func New(cfg config.Config, st *store.Store, set *settings.Store, au *auth.Store, jb *jobs.Store, mus *music.Store, uploads *upload.Manager, art *artwork.Service, subs *subtitles.Service, tc *transcode.JobHandler, sessions *transcode.SessionManager) *Server {
	return &Server{cfg: cfg, store: st, settings: set, auth: au, jobs: jb, music: mus, uploads: uploads, artwork: art, subtitles: subs, transcode: tc, sessions: sessions}
}

func (s *Server) Handler() http.Handler {
	sessions := auth.NewMiddleware(s.auth)
	authModule := auth.NewModule(s.auth, s.cfg, s.artwork)
	adminTitles := api.NewAdminTitles(s.store, s.jobs, s.artwork)
	adminSettings := api.NewAdminSettings(s.settings)
	adminLibraries := api.NewAdminLibraries(s.store, s.jobs)
	adminJobs := jobs.NewAdminJobs(s.jobs)
	catalog := api.NewCatalog(s.store, s.settings, s.artwork.Store, s.music)
	stream := api.NewStream(s.store, s.settings, s.jobs, s.cfg.DataDir, s.sessions, s.cfg.FFmpegPath)
	progress := api.NewProgress(s.store)
	transcodeAPI := api.NewAdminTranscode(s.store, s.settings, s.jobs, s.transcode, s.cfg.FFmpegPath)
	musicModule := music.NewModule(s.music, s.settings, s.artwork)
	adminStorage := api.NewAdminStorage(s.store, s.jobs, s.cfg.DataDir)
	sysStats := api.NewSysStats()
	theme := api.NewTheme(s.settings)
	artworkAPI := artwork.NewHandlers(s.artwork)
	subtitlesAPI := api.NewSubtitles(s.store, s.subtitles)
	uploadsAPI := api.NewAdminUploads(s.store, s.uploads)
	metadataAPI := api.NewAdminMetadata(s.store, s.settings, s.jobs)

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
			p.Get("/genres", func(w http.ResponseWriter, r *http.Request) {
				genres, err := s.store.ListGenres(r.Context())
				if err != nil {
					httpx.Internal(w, err)
					return
				}
				httpx.JSON(w, http.StatusOK, genres)
			})

			p.Get("/home", catalog.Home)
			p.Get("/titles", catalog.Browse)
			p.Get("/titles/{slug}", catalog.Title)
			p.Get("/search", catalog.Search)

			p.Get("/stream/{id}", stream.Serve)
			p.Get("/stream/{id}/hls/master.m3u8", stream.HLSMaster)
			p.Get("/stream/{id}/hls/{variant}/{file}", stream.HLSFile)
			p.Post("/stream/{id}/sessions", stream.CreateSession)
			p.Get("/stream/sessions/{sid}/{file}", stream.SessionFile)
			p.Post("/stream/sessions/{sid}/keepalive", stream.SessionKeepalive)
			p.Get("/playback/{kind}/{id}", stream.Playback)
			artworkAPI.MountUser(p)
			p.Get("/subtitles/{id}.vtt", subtitlesAPI.Serve)

			p.Get("/features", func(w http.ResponseWriter, r *http.Request) {
				httpx.JSON(w, http.StatusOK, flags.Load(r.Context(), s.settings))
			})

			// music routes (incl. track playlists) gate themselves on the feature toggle
			musicModule.MountUser(p)

			p.Put("/progress", progress.Put)
			p.Post("/progress", progress.Put) // sendBeacon can only POST
			p.Get("/me/continue-watching", progress.ContinueWatching)
			p.Get("/me/watchlist", progress.WatchlistGet)
			p.Put("/me/watchlist/{titleId}", progress.WatchlistPut)
			p.Delete("/me/watchlist/{titleId}", progress.WatchlistDelete)
		})

		// admin routes
		v1.Route("/admin", func(adm chi.Router) {
			adm.Use(auth.RequireAdmin)

			adm.Get("/library", adminTitles.Library)
			adm.Post("/titles", adminTitles.Create)
			adm.Post("/titles/bulk", adminTitles.Bulk)
			adm.Get("/titles/{id}", adminTitles.Get)
			adm.Patch("/titles/{id}", adminTitles.Update)
			adm.Delete("/titles/{id}", adminTitles.Delete)
			adm.Post("/titles/{id}/seasons", adminTitles.CreateSeason)
			adm.Delete("/seasons/{id}", adminTitles.DeleteSeason)
			adm.Post("/seasons/{id}/episodes", adminTitles.CreateEpisode)
			adm.Patch("/episodes/{id}", adminTitles.UpdateEpisode)
			adm.Delete("/episodes/{id}", adminTitles.DeleteEpisode)

			musicModule.MountAdmin(adm)

			authModule.MountAdmin(adm)

			adm.Get("/settings", adminSettings.Get)
			adm.Put("/settings", adminSettings.Put)

			adm.Get("/libraries", adminLibraries.List)
			adm.Post("/libraries", adminLibraries.Create)
			adm.Delete("/libraries/{id}", adminLibraries.Delete)
			adm.Post("/libraries/{id}/scan", adminLibraries.Scan)
			adm.Post("/libraries/scan-all", adminLibraries.ScanAll)

			adminJobs.MountAdmin(adm)

			adm.Get("/uploads", uploadsAPI.List)
			adm.Post("/uploads", uploadsAPI.Create)
			adm.Get("/uploads/{id}", uploadsAPI.Get)
			adm.Put("/uploads/{id}", uploadsAPI.Append)
			adm.Post("/uploads/{id}/complete", uploadsAPI.Complete)
			adm.Delete("/uploads/{id}", uploadsAPI.Abort)

			adm.Get("/metadata/search", metadataAPI.Search)
			adm.Post("/titles/{id}/metadata/apply", metadataAPI.Apply)
			adm.Get("/titles/{id}/metadata/seasons", metadataAPI.Seasons)
			adm.Post("/titles/{id}/metadata/import-episodes", metadataAPI.ImportEpisodes)

			artworkAPI.MountAdmin(adm)

			adm.Get("/media-files/{id}/subtitles", subtitlesAPI.ListForMediaFile)
			adm.Post("/media-files/{id}/subtitles", subtitlesAPI.Upload)
			adm.Delete("/subtitles/{id}", subtitlesAPI.Delete)

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
