package couch

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"couchverse/internal/flags"
	"couchverse/internal/httpx"
	"couchverse/internal/settings"
)

// Module mounts the couch HTTP routes. The WebSocket route is added in ws.go.
type Module struct {
	hub      *Hub
	handlers *Handlers
	settings *settings.Store
}

func NewModule(hub *Hub, set *settings.Store) *Module {
	return &Module{hub: hub, handlers: &Handlers{hub: hub}, settings: set}
}

// MountUser registers the public couch routes (anonymous followers must reach
// them); each handler enforces its own auth. The whole group is hidden when the
// couchEnabled flag is off.
func (m *Module) MountUser(r chi.Router) {
	r.Group(func(c chi.Router) {
		c.Use(flags.RequireCouch(m.settings))
		c.Post("/couch", m.handlers.Create)
		c.Post("/couch/{token}/join", m.handlers.Join)
		c.Post("/couch/{token}/leave", m.handlers.Leave)
		c.Post("/couch/{token}/end", m.handlers.End)
		c.Get("/couch/{token}/playback", withLang(m.handlers.Playback))
	})
}

// withLang threads the ?lang= display language onto the request context so a
// follower's player payload shows titles in their language.
func withLang(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h(w, r.WithContext(httpx.WithLang(r.Context(), httpx.Lang(r))))
	}
}
