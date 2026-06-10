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
	"couchverse/internal/artwork"
	"couchverse/internal/auth"
	"couchverse/internal/config"
	"couchverse/internal/features"
	"couchverse/internal/httpx"
	"couchverse/internal/store"
	"couchverse/internal/subtitles"
	"couchverse/internal/transcode"
	"couchverse/internal/upload"
	"couchverse/web"
)

type Server struct {
	cfg       config.Config
	store     *store.Store
	uploads   *upload.Manager
	artwork   *artwork.Service
	subtitles *subtitles.Service
	transcode *transcode.JobHandler
	sessions  *transcode.SessionManager
}

func New(cfg config.Config, st *store.Store, uploads *upload.Manager, art *artwork.Service, subs *subtitles.Service, tc *transcode.JobHandler, sessions *transcode.SessionManager) *Server {
	return &Server{cfg: cfg, store: st, uploads: uploads, artwork: art, subtitles: subs, transcode: tc, sessions: sessions}
}

func (s *Server) Handler() http.Handler {
	sessions := auth.NewMiddleware(s.store)
	authAPI := api.NewAuth(s.store, s.cfg)
	adminTitles := api.NewAdminTitles(s.store, s.artwork)
	adminUsers := api.NewAdminUsers(s.store)
	adminSettings := api.NewAdminSettings(s.store)
	adminLibraries := api.NewAdminLibraries(s.store)
	adminJobs := api.NewAdminJobs(s.store)
	catalog := api.NewCatalog(s.store)
	stream := api.NewStream(s.store, s.cfg.DataDir, s.sessions, s.cfg.FFmpegPath)
	progress := api.NewProgress(s.store)
	transcodeAPI := api.NewAdminTranscode(s.store, s.transcode, s.cfg.FFmpegPath)
	music := api.NewMusic(s.store)
	playlists := api.NewPlaylists(s.store)
	adminStorage := api.NewAdminStorage(s.store, s.cfg.DataDir)
	adminMusic := api.NewAdminMusic(s.store, s.artwork)
	profile := api.NewProfile(s.store, s.artwork)
	artworkAPI := api.NewArtwork(s.store, s.artwork)
	subtitlesAPI := api.NewSubtitles(s.store, s.subtitles)
	uploadsAPI := api.NewAdminUploads(s.store, s.uploads)
	metadataAPI := api.NewAdminMetadata(s.store)

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

		v1.Post("/auth/login", authAPI.Login)
		v1.Post("/auth/logout", authAPI.Logout)

		// authenticated routes
		v1.Group(func(p chi.Router) {
			p.Use(auth.RequireAuth)
			p.Get("/auth/me", authAPI.Me)
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
			p.Get("/artwork/{id}", artworkAPI.Serve)
			p.Get("/subtitles/{id}.vtt", subtitlesAPI.Serve)

			p.Get("/features", func(w http.ResponseWriter, r *http.Request) {
				httpx.JSON(w, http.StatusOK, features.Load(r.Context(), s.store))
			})

			// music (incl. track playlists) sits behind the feature toggle
			p.Group(func(m chi.Router) {
				m.Use(features.RequireMusic(s.store))
				m.Get("/music", music.Home)
				m.Get("/music/albums/{id}", music.Album)
				m.Get("/music/artists/{id}", music.Artist)
				m.Post("/plays", music.Scrobble)

				m.Get("/me/playlists", playlists.List)
				m.Post("/me/playlists", playlists.Create)
				m.Get("/me/playlists/{id}", playlists.Get)
				m.Patch("/me/playlists/{id}", playlists.Rename)
				m.Delete("/me/playlists/{id}", playlists.Delete)
				m.Post("/me/playlists/{id}/tracks", playlists.AddTrack)
				m.Delete("/me/playlists/{id}/tracks/{entryId}", playlists.RemoveEntry)
				m.Put("/me/playlists/{id}/order", playlists.Reorder)
			})

			p.Patch("/me/profile", profile.Update)
			p.Post("/me/avatar", profile.SetAvatar)
			p.Delete("/me/avatar", profile.DeleteAvatar)

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

			adm.Group(func(m chi.Router) {
				m.Use(features.RequireMusic(s.store))
				m.Get("/music", adminMusic.List)
				m.Get("/albums/{id}", adminMusic.Get)
				m.Patch("/albums/{id}", adminMusic.Update)
				m.Delete("/albums/{id}", adminMusic.Delete)
				m.Patch("/tracks/{id}", adminMusic.UpdateTrack)
				m.Delete("/tracks/{id}", adminMusic.DeleteTrack)
			})

			adm.Get("/users", adminUsers.List)
			adm.Post("/users", adminUsers.Create)
			adm.Patch("/users/{id}", adminUsers.Update)
			adm.Delete("/users/{id}", adminUsers.Delete)

			adm.Get("/settings", adminSettings.Get)
			adm.Put("/settings", adminSettings.Put)

			adm.Get("/libraries", adminLibraries.List)
			adm.Post("/libraries", adminLibraries.Create)
			adm.Delete("/libraries/{id}", adminLibraries.Delete)
			adm.Post("/libraries/{id}/scan", adminLibraries.Scan)
			adm.Post("/libraries/scan-all", adminLibraries.ScanAll)

			adm.Get("/jobs", adminJobs.List)
			adm.Post("/jobs/{id}/retry", adminJobs.Retry)
			adm.Post("/jobs/{id}/cancel", adminJobs.Cancel)

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

			adm.Post("/artwork", artworkAPI.Upload)
			adm.Delete("/artwork/{id}", artworkAPI.Delete)

			adm.Get("/media-files/{id}/subtitles", subtitlesAPI.ListForMediaFile)
			adm.Post("/media-files/{id}/subtitles", subtitlesAPI.Upload)
			adm.Delete("/subtitles/{id}", subtitlesAPI.Delete)

			adm.Get("/storage", adminStorage.Get)
			adm.Get("/overview", adminStorage.Overview)
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
