package playback

import (
	"github.com/go-chi/chi/v5"
)

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
	r.Get("/playback/{kind}/{id}", m.stream.Playback)
}

func (m *Module) MountAdmin(r chi.Router) {
	r.Get("/transcode/info", m.admin.Info)
	r.Get("/transcode/active", m.admin.Active)
	r.Post("/media-files/{id}/transcode", m.admin.Enqueue)
	r.Get("/media-files/{id}/variants", m.admin.ListVariants)
	r.Delete("/transcode-variants/{id}", m.admin.DeleteVariant)
}
