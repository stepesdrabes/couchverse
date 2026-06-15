package catalog

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"couchverse/internal/feature/analytics"
	"couchverse/internal/feature/artwork"
	"couchverse/internal/feature/jobs"
	"couchverse/internal/feature/music"
	"couchverse/internal/httpx"
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

// withLang stores the ?lang= display language on the request context so the
// catalog stores localize names/overviews. Admin and background-job paths leave
// it unset, so they always see the base (default-language) text.
func withLang(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h(w, r.WithContext(httpx.WithLang(r.Context(), httpx.Lang(r))))
	}
}

func (m *Module) MountUser(r chi.Router) {
	r.Get("/genres", withLang(m.handlers.Genres))
	r.Get("/home", withLang(m.handlers.Home))
	r.Get("/titles", withLang(m.handlers.Browse))
	r.Get("/titles/{slug}", withLang(m.handlers.Title))
	r.Get("/search", withLang(m.handlers.Search))

	r.Put("/progress", m.progress.Put)
	r.Post("/progress", m.progress.Put) // sendBeacon can only POST
	r.Get("/me/continue-watching", withLang(m.progress.ContinueWatching))
	r.Get("/me/watchlist", withLang(m.progress.WatchlistGet))
	r.Put("/me/watchlist/{titleId}", m.progress.WatchlistPut)
	r.Delete("/me/watchlist/{titleId}", m.progress.WatchlistDelete)
}

func (m *Module) MountAdmin(r chi.Router) {
	r.Get("/library", m.admin.Library)
	r.Post("/titles", m.admin.Create)
	r.Post("/titles/bulk", m.admin.Bulk)
	r.Get("/titles/{id}", m.admin.Get)
	r.Get("/titles/{id}/storage", m.admin.Storage)
	r.Patch("/titles/{id}", m.admin.Update)
	r.Patch("/titles/{id}/translations/{lang}", m.admin.SetTranslation)
	r.Delete("/titles/{id}/languages/{lang}", m.admin.DeleteLanguage)
	r.Delete("/titles/{id}", m.admin.Delete)
	r.Post("/titles/{id}/seasons", m.admin.CreateSeason)
	r.Delete("/seasons/{id}", m.admin.DeleteSeason)
	r.Post("/seasons/{id}/episodes", m.admin.CreateEpisode)
	r.Patch("/episodes/{id}", m.admin.UpdateEpisode)
	r.Get("/episodes/{id}/translations", m.admin.EpisodeTranslations)
	r.Patch("/episodes/{id}/translations/{lang}", m.admin.SetEpisodeTranslation)
	r.Delete("/episodes/{id}", m.admin.DeleteEpisode)
}
