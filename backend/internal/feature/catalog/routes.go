package catalog

import (
	"github.com/go-chi/chi/v5"

	"couchverse/internal/feature/analytics"
	"couchverse/internal/feature/artwork"
	"couchverse/internal/feature/jobs"
	"couchverse/internal/feature/music"
	"couchverse/internal/settings"
)

// Module bundles the browse, progress and title-admin handlers.
type Module struct {
	handlers *Handlers
	progress *Progress
	admin    *AdminHandlers
}

func NewModule(st *Store, set *settings.Store, art *artwork.Service, mus *music.Store, jb *jobs.Store, an *analytics.Store) *Module {
	return &Module{
		handlers: NewHandlers(st, set, art.Store, mus),
		progress: NewProgress(st, an),
		admin:    NewAdminHandlers(st, jb, art),
	}
}

func (m *Module) MountUser(r chi.Router) {
	r.Get("/genres", m.handlers.Genres)
	r.Get("/home", m.handlers.Home)
	r.Get("/titles", m.handlers.Browse)
	r.Get("/titles/{slug}", m.handlers.Title)
	r.Get("/search", m.handlers.Search)

	r.Put("/progress", m.progress.Put)
	r.Post("/progress", m.progress.Put) // sendBeacon can only POST
	r.Get("/me/continue-watching", m.progress.ContinueWatching)
	r.Get("/me/watchlist", m.progress.WatchlistGet)
	r.Put("/me/watchlist/{titleId}", m.progress.WatchlistPut)
	r.Delete("/me/watchlist/{titleId}", m.progress.WatchlistDelete)
}

func (m *Module) MountAdmin(r chi.Router) {
	r.Get("/library", m.admin.Library)
	r.Post("/titles", m.admin.Create)
	r.Post("/titles/bulk", m.admin.Bulk)
	r.Get("/titles/{id}", m.admin.Get)
	r.Patch("/titles/{id}", m.admin.Update)
	r.Delete("/titles/{id}", m.admin.Delete)
	r.Post("/titles/{id}/seasons", m.admin.CreateSeason)
	r.Delete("/seasons/{id}", m.admin.DeleteSeason)
	r.Post("/seasons/{id}/episodes", m.admin.CreateEpisode)
	r.Patch("/episodes/{id}", m.admin.UpdateEpisode)
	r.Delete("/episodes/{id}", m.admin.DeleteEpisode)
}
