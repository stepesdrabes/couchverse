package catalog

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"couchverse/internal/feature/analytics"
	"couchverse/internal/feature/artwork"
	"couchverse/internal/feature/jobs"
	"couchverse/internal/httpx"
	"couchverse/internal/settings"
)

// Module bundles the browse, progress and title-admin handlers.
type Module struct {
	handlers *Handlers
	progress *Progress
	admin    *AdminHandlers
}

func NewModule(st *Store, set *settings.Store, art *artwork.Service, jb *jobs.Store, an *analytics.Store) *Module {
	return &Module{
		handlers: NewHandlers(st, set, art.Store),
		progress: NewProgress(st, an),
		admin:    NewAdminHandlers(st, jb, art),
	}
}

const tag httpx.Tag = "catalog"

func (m *Module) Register(rt httpx.Routes) {
	huma.Register(rt.User, httpx.Localized(tag.Op("listGenres", http.MethodGet, "/genres")), m.handlers.Genres)
	huma.Register(rt.User, httpx.Localized(tag.Op("getHome", http.MethodGet, "/home")), m.handlers.Home)
	huma.Register(rt.User, httpx.Localized(tag.Op("browseTitles", http.MethodGet, "/titles")), m.handlers.Browse)
	huma.Register(rt.User, httpx.Localized(tag.Op("getTitle", http.MethodGet, "/titles/{slug}")), m.handlers.Title)
	huma.Register(rt.User, httpx.Localized(tag.Op("search", http.MethodGet, "/search")), m.handlers.Search)

	huma.Register(rt.User, tag.NoContent("saveProgress", http.MethodPut, "/progress"), m.progress.Put)
	// sendBeacon can only POST
	huma.Register(rt.User, tag.NoContent("saveProgressBeacon", http.MethodPost, "/progress"), m.progress.Put)
	huma.Register(rt.User, httpx.Localized(tag.Op("listContinueWatching", http.MethodGet, "/me/continue-watching")), m.progress.ContinueWatching)
	huma.Register(rt.User, httpx.Localized(tag.Op("listWatchlist", http.MethodGet, "/me/watchlist")), m.progress.WatchlistGet)
	huma.Register(rt.User, tag.NoContent("addToWatchlist", http.MethodPut, "/me/watchlist/{titleId}"), m.progress.WatchlistPut)
	huma.Register(rt.User, tag.NoContent("removeFromWatchlist", http.MethodDelete, "/me/watchlist/{titleId}"), m.progress.WatchlistDelete)

	huma.Register(rt.Admin, tag.Op("adminListLibrary", http.MethodGet, "/library"), m.admin.Library)
	huma.Register(rt.Admin, tag.Op("adminCreateTitle", http.MethodPost, "/titles"), m.admin.Create)
	huma.Register(rt.Admin, tag.NoContent("adminBulkTitles", http.MethodPost, "/titles/bulk"), m.admin.Bulk)
	huma.Register(rt.Admin, tag.Op("adminGetTitle", http.MethodGet, "/titles/{id}"), m.admin.Get)
	huma.Register(rt.Admin, tag.Op("adminGetTitleStorage", http.MethodGet, "/titles/{id}/storage"), m.admin.Storage)
	huma.Register(rt.Admin, tag.Op("adminUpdateTitle", http.MethodPatch, "/titles/{id}"), m.admin.Update)
	huma.Register(rt.Admin, tag.NoContent("adminSetTitleTranslation", http.MethodPatch, "/titles/{id}/translations/{lang}"), m.admin.SetTranslation)
	huma.Register(rt.Admin, tag.NoContent("adminDeleteTitleLanguage", http.MethodDelete, "/titles/{id}/languages/{lang}"), m.admin.DeleteLanguage)
	huma.Register(rt.Admin, tag.NoContent("adminDeleteTitle", http.MethodDelete, "/titles/{id}"), m.admin.Delete)
	huma.Register(rt.Admin, tag.Op("adminCreateSeason", http.MethodPost, "/titles/{id}/seasons"), m.admin.CreateSeason)
	huma.Register(rt.Admin, tag.NoContent("adminDeleteSeason", http.MethodDelete, "/seasons/{id}"), m.admin.DeleteSeason)
	huma.Register(rt.Admin, tag.Op("adminCreateEpisode", http.MethodPost, "/seasons/{id}/episodes"), m.admin.CreateEpisode)
	huma.Register(rt.Admin, tag.Op("adminUpdateEpisode", http.MethodPatch, "/episodes/{id}"), m.admin.UpdateEpisode)
	huma.Register(rt.Admin, tag.Op("adminGetEpisodeTranslations", http.MethodGet, "/episodes/{id}/translations"), m.admin.EpisodeTranslations)
	huma.Register(rt.Admin, tag.NoContent("adminSetEpisodeTranslation", http.MethodPatch, "/episodes/{id}/translations/{lang}"), m.admin.SetEpisodeTranslation)
	huma.Register(rt.Admin, tag.NoContent("adminDeleteEpisode", http.MethodDelete, "/episodes/{id}"), m.admin.DeleteEpisode)
}
