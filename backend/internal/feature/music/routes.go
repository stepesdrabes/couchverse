package music

import (
	"github.com/go-chi/chi/v5"

	"couchverse/internal/feature/artwork"
	"couchverse/internal/flags"
	"couchverse/internal/settings"
)

// Module bundles the music handlers and owns the feature-flag gating of its
// routes.
type Module struct {
	handlers  *Handlers
	playlists *Playlists
	admin     *AdminHandlers
	settings  *settings.Store
}

func NewModule(st *Store, set *settings.Store, art *artwork.Service) *Module {
	return &Module{
		handlers:  NewHandlers(st),
		playlists: NewPlaylists(st),
		admin:     NewAdminHandlers(st, art),
		settings:  set,
	}
}

func (mod *Module) MountUser(r chi.Router) {
	r.Group(func(m chi.Router) {
		m.Use(flags.RequireMusic(mod.settings))
		m.Get("/music", mod.handlers.Home)
		m.Get("/music/albums/{id}", mod.handlers.Album)
		m.Get("/music/artists/{id}", mod.handlers.Artist)
		m.Post("/plays", mod.handlers.Scrobble)

		m.Get("/me/playlists", mod.playlists.List)
		m.Post("/me/playlists", mod.playlists.Create)
		m.Get("/me/playlists/{id}", mod.playlists.Get)
		m.Patch("/me/playlists/{id}", mod.playlists.Rename)
		m.Delete("/me/playlists/{id}", mod.playlists.Delete)
		m.Post("/me/playlists/{id}/tracks", mod.playlists.AddTrack)
		m.Delete("/me/playlists/{id}/tracks/{entryId}", mod.playlists.RemoveEntry)
		m.Put("/me/playlists/{id}/order", mod.playlists.Reorder)
	})
}

func (mod *Module) MountAdmin(r chi.Router) {
	r.Group(func(m chi.Router) {
		m.Use(flags.RequireMusic(mod.settings))
		m.Get("/music", mod.admin.List)
		m.Get("/albums/{id}", mod.admin.Get)
		m.Patch("/albums/{id}", mod.admin.Update)
		m.Delete("/albums/{id}", mod.admin.Delete)
		m.Patch("/tracks/{id}", mod.admin.UpdateTrack)
		m.Delete("/tracks/{id}", mod.admin.DeleteTrack)
	})
}
