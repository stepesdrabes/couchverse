package couch

import (
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"couchverse/internal/flags"
	"couchverse/internal/httpx"
	"couchverse/internal/settings"
)

// Module mounts the couch HTTP + WebSocket routes.
type Module struct {
	hub      *Hub
	handlers *Handlers
	settings *settings.Store
}

func NewModule(hub *Hub, set *settings.Store) *Module {
	return &Module{
		hub:      hub,
		handlers: &Handlers{hub: hub, joinRate: newRateLimiter(10, time.Minute)},
		settings: set,
	}
}

// Register adds the couch routes to the public group (anonymous followers must
// reach them); each handler enforces its own auth. The whole group is hidden
// when the couchEnabled flag is off.
func (m *Module) Register(rt httpx.Routes) {
	tags := []string{"couch"}
	api := huma.NewGroup(rt.Public)
	api.UseMiddleware(httpx.Guard(api, flags.CouchOn(m.settings)))
	httpx.Raw(api, huma.Operation{OperationID: "createCouch", Method: http.MethodPost, Path: "/couch", Tags: tags}, m.handlers.Create)
	httpx.Raw(api, huma.Operation{OperationID: "getCouchInfo", Method: http.MethodGet, Path: "/couch/{token}/info", Tags: tags}, withLang(m.handlers.Info))
	httpx.Raw(api, huma.Operation{OperationID: "joinCouch", Method: http.MethodPost, Path: "/couch/{token}/join", Tags: tags}, m.handlers.Join)
	httpx.Raw(api, huma.Operation{OperationID: "leaveCouch", Method: http.MethodPost, Path: "/couch/{token}/leave", Tags: tags}, m.handlers.Leave)
	httpx.Raw(api, huma.Operation{OperationID: "endCouch", Method: http.MethodPost, Path: "/couch/{token}/end", Tags: tags}, m.handlers.End)
	httpx.Raw(api, huma.Operation{OperationID: "getCouchPlayback", Method: http.MethodGet, Path: "/couch/{token}/playback", Tags: tags}, withLang(m.handlers.Playback))
	httpx.Raw(api, huma.Operation{OperationID: "couchSocket", Method: http.MethodGet, Path: "/couch/{token}/ws", Tags: tags}, m.handlers.WS)
}

// withLang threads the ?lang= display language onto the request context so a
// follower's player payload shows titles in their language.
func withLang(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h(w, r.WithContext(httpx.WithLang(r.Context(), httpx.Lang(r))))
	}
}
