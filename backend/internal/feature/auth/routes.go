package auth

import (
	"github.com/go-chi/chi/v5"

	"couchverse/internal/config"
	"couchverse/internal/feature/artwork"
)

// Module bundles the auth, profile and user-admin handlers.
type Module struct {
	handlers *Handlers
	profile  *Profile
	admin    *AdminUsers
}

func NewModule(st *Store, cfg config.Config, art *artwork.Service) *Module {
	return &Module{
		handlers: NewHandlers(st, cfg),
		profile:  NewProfile(st, art),
		admin:    NewAdminUsers(st),
	}
}

// MountPublic mounts the routes that must work without a session.
func (m *Module) MountPublic(r chi.Router) {
	r.Post("/auth/login", m.handlers.Login)
	r.Post("/auth/logout", m.handlers.Logout)
}

func (m *Module) MountUser(r chi.Router) {
	r.Get("/auth/me", m.handlers.Me)
	r.Patch("/me/profile", m.profile.Update)
	r.Get("/me/preferences", m.profile.Preferences)
	r.Put("/me/preferences", m.profile.UpdatePreferences)
	r.Post("/me/avatar", m.profile.SetAvatar)
	r.Delete("/me/avatar", m.profile.DeleteAvatar)
}

func (m *Module) MountAdmin(r chi.Router) {
	r.Get("/users", m.admin.List)
	r.Post("/users", m.admin.Create)
	r.Patch("/users/{id}", m.admin.Update)
	r.Delete("/users/{id}", m.admin.Delete)
}
