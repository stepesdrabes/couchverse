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
	"couchverse/internal/auth"
	"couchverse/internal/config"
	"couchverse/internal/httpx"
	"couchverse/internal/store"
	"couchverse/web"
)

type Server struct {
	cfg   config.Config
	store *store.Store
}

func New(cfg config.Config, st *store.Store) *Server {
	return &Server{cfg: cfg, store: st}
}

func (s *Server) Handler() http.Handler {
	sessions := auth.NewMiddleware(s.store)
	authAPI := api.NewAuth(s.store, s.cfg)
	adminTitles := api.NewAdminTitles(s.store)
	adminUsers := api.NewAdminUsers(s.store)
	adminSettings := api.NewAdminSettings(s.store)
	adminLibraries := api.NewAdminLibraries(s.store)
	adminJobs := api.NewAdminJobs(s.store)

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
