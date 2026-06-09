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
	"github.com/jackc/pgx/v5/pgxpool"

	"couchverse/internal/config"
	"couchverse/internal/httpx"
	"couchverse/web"
)

type Server struct {
	cfg  config.Config
	pool *pgxpool.Pool
}

func New(cfg config.Config, pool *pgxpool.Pool) *Server {
	return &Server{cfg: cfg, pool: pool}
}

func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(requestLogger)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", s.handleHealthz)

	r.Route("/api/v1", func(api chi.Router) {
		api.NotFound(func(w http.ResponseWriter, _ *http.Request) {
			httpx.NotFound(w)
		})
	})

	r.NotFound(spaHandler())
	return r
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.pool.Ping(ctx); err != nil {
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
