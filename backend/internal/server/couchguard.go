package server

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"

	"couchverse/internal/feature/auth"
	"couchverse/internal/httpx"
)

// authOrCouch admits a request if it is logged in or if the couch Hub authorizes
// this anonymous viewer to stream the requested media - a live couch session
// currently playing exactly that media file. When couch is absent or its flag is
// off, this collapses to auth.SignedIn.
func (s *Server) authOrCouch(api huma.API) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		r, _ := humachi.Unwrap(ctx)
		if auth.UserFrom(r.Context()) == nil && (s.Couch == nil || !s.Couch.AllowsAnon(r, s.streamMediaFileID(r))) {
			_ = huma.WriteErr(api, ctx, http.StatusUnauthorized, "authentication required")
			return
		}
		next(ctx)
	}
}

// streamMediaFileID derives the media file id a stream request targets. Most
// stream routes carry it as the {id} path param; the JIT segment routes carry
// only {sid}, which the shared playback SessionManager resolves to its media
// file. No couch/playback import is needed here - SessionManager is a Server
// field and Session.MediaFileID is exported.
func (s *Server) streamMediaFileID(r *http.Request) string {
	if sid := chi.URLParam(r, "sid"); sid != "" {
		if sess := s.Sessions.Get(sid); sess != nil {
			return sess.MediaFileID
		}
		return ""
	}
	return httpx.UUID(r, "id")
}
