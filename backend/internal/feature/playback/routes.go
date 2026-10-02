package playback

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"

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

func (m *Module) Register(rt httpx.Routes) {
	tags := []string{"playback"}
	stream := rt.Stream
	httpx.Raw(stream, huma.Operation{OperationID: "streamMediaFile", Method: http.MethodGet, Path: "/stream/{id}", Tags: tags}, m.stream.Serve)
	httpx.Raw(stream, huma.Operation{OperationID: "getStreamFrame", Method: http.MethodGet, Path: "/stream/{id}/frame", Tags: tags}, m.stream.Frame)
	httpx.Raw(stream, huma.Operation{OperationID: "getHlsMaster", Method: http.MethodGet, Path: "/stream/{id}/hls/master.m3u8", Tags: tags}, m.stream.HLSMaster)
	httpx.Raw(stream, huma.Operation{OperationID: "getHlsFile", Method: http.MethodGet, Path: "/stream/{id}/hls/{variant}/{file}", Tags: tags}, m.stream.HLSFile)
	httpx.Raw(stream, huma.Operation{OperationID: "createStreamSession", Method: http.MethodPost, Path: "/stream/{id}/sessions", Tags: tags}, m.stream.CreateSession)
	httpx.Raw(stream, huma.Operation{OperationID: "getStreamSessionFile", Method: http.MethodGet, Path: "/stream/sessions/{sid}/{file}", Tags: tags}, m.stream.SessionFile)
	httpx.Raw(stream, huma.Operation{OperationID: "keepStreamSessionAlive", Method: http.MethodPost, Path: "/stream/sessions/{sid}/keepalive", Tags: tags}, m.stream.SessionKeepalive)
	httpx.Raw(stream, huma.Operation{OperationID: "getPlayback", Method: http.MethodGet, Path: "/playback/{kind}/{id}", Tags: tags}, withLang(m.stream.Playback))

	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminGetTranscodeInfo", Method: http.MethodGet, Path: "/transcode/info", Tags: tags}, m.admin.Info)
	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminListActiveTranscodes", Method: http.MethodGet, Path: "/transcode/active", Tags: tags}, m.admin.Active)
	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminEnqueueTranscode", Method: http.MethodPost, Path: "/media-files/{id}/transcode", Tags: tags}, m.admin.Enqueue)
	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminListVariants", Method: http.MethodGet, Path: "/media-files/{id}/variants", Tags: tags}, m.admin.ListVariants)
	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminDeleteVariant", Method: http.MethodDelete, Path: "/transcode-variants/{id}", Tags: tags}, m.admin.DeleteVariant)
}
