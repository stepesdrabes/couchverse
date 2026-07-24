package ranks

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"couchverse/internal/flags"
	"couchverse/internal/httpx"
	"couchverse/internal/settings"
)

// Module mounts the progression routes. The whole group is hidden when the
// rankingsEnabled flag is off, mirroring music and couch.
type Module struct {
	handlers *Handlers
	admin    *AdminRanks
	settings *settings.Store
}

func NewModule(st *Store, set *settings.Store) *Module {
	return &Module{handlers: NewHandlers(st, set), admin: NewAdminRanks(st, set), settings: set}
}

func (m *Module) MountUser(r chi.Router) {
	r.Group(func(g chi.Router) {
		g.Use(flags.RequireRankings(m.settings))
		g.Get("/me/stats", withLang(m.handlers.MyStats))
		g.Post("/me/achievements/check", m.handlers.Check)
		g.Get("/users/{username}/profile", withLang(m.handlers.PublicProfile))
		g.Get("/leaderboard", m.handlers.Leaderboard)
	})
}

// MountAdmin registers the progression admin. Deliberately not flag-gated: an
// admin has to be able to inspect and retune ranks in order to turn them back on.
func (m *Module) MountAdmin(r chi.Router) {
	r.Get("/ranks", m.admin.Overview)
	r.Put("/ranks/config", m.admin.PutConfig)
}

// withLang threads the ?lang= display language onto the request context so a
// profile's title names and favourite genre read in the visitor's language.
func withLang(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h(w, r.WithContext(httpx.WithLang(r.Context(), httpx.Lang(r))))
	}
}
