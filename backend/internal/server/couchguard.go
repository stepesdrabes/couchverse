package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"couchverse/internal/feature/auth"
	"couchverse/internal/httpx"
)

// requireAuthOrCouch admits a request if it is logged in (identical to today) or
// if the couch Hub authorizes this anonymous viewer to stream the requested
// media - a live couch session currently playing exactly that media file. When
// couch is absent or its flag is off, this collapses to auth.RequireAuth.
func (s *Server) requireAuthOrCouch(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth.UserFrom(r.Context()) != nil {
			next.ServeHTTP(w, r)
			return
		}
		if s.couch == nil || !s.couch.AllowsAnon(r, s.streamMediaFileID(r)) {
			httpx.Error(w, http.StatusUnauthorized, "unauthorized", "authentication required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// streamMediaFileID derives the media file id a stream request targets. Most
// stream routes carry it as the {id} path param; the JIT segment routes carry
// only {sid}, which the shared playback SessionManager resolves to its media
// file. No couch/playback import is needed here - SessionManager is a Server
// field and Session.MediaFileID is exported.
func (s *Server) streamMediaFileID(r *http.Request) string {
	if sid := chi.URLParam(r, "sid"); sid != "" {
		if sess := s.sessions.Get(sid); sess != nil {
			return sess.MediaFileID
		}
		return ""
	}
	return httpx.UUID(r, "id")
}
