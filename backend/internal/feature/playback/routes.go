package playback

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"couchverse/internal/httpx"
)

// withLang puts the ?lang= display language on the request context so the
// player's title and episode names come back in the viewer's language.
func withLang(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h(w, r.WithContext(httpx.WithLang(r.Context(), httpx.Lang(r))))
	}
}

// Module bundles the streaming and transcode-admin handlers.
type Module struct {
	stream *Stream
	admin  *AdminTranscode
}

func NewModule(stream *Stream, admin *AdminTranscode) *Module {
	return &Module{stream: stream, admin: admin}
}

func (m *Module) MountUser(r chi.Router) {
	r.Get("/stream/{id}", m.stream.Serve)
	r.Get("/stream/{id}/frame", m.stream.Frame)
	r.Get("/stream/{id}/hls/master.m3u8", m.stream.HLSMaster)
	r.Get("/stream/{id}/hls/{variant}/{file}", m.stream.HLSFile)
	r.Post("/stream/{id}/sessions", m.stream.CreateSession)
	r.Get("/stream/sessions/{sid}/{file}", m.stream.SessionFile)
	r.Post("/stream/sessions/{sid}/keepalive", m.stream.SessionKeepalive)
	r.Get("/playback/{kind}/{id}", withLang(m.stream.Playback))
}

func (m *Module) MountAdmin(r chi.Router) {
	r.Get("/transcode/info", m.admin.Info)
	r.Get("/transcode/active", m.admin.Active)
	r.Post("/media-files/{id}/transcode", m.admin.Enqueue)
	r.Get("/media-files/{id}/variants", m.admin.ListVariants)
	r.Delete("/transcode-variants/{id}", m.admin.DeleteVariant)
}
