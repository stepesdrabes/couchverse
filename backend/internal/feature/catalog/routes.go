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

// withLang stores the ?lang= display language on the request context so the
// catalog stores localize names/overviews. Admin and background-job paths leave
// it unset, so they always see the base (default-language) text.
func withLang(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h(w, r.WithContext(httpx.WithLang(r.Context(), httpx.Lang(r))))
	}
}

func (m *Module) Register(rt httpx.Routes) {
	tags := []string{"catalog"}
	httpx.Raw(rt.User, huma.Operation{OperationID: "listGenres", Method: http.MethodGet, Path: "/genres", Tags: tags}, withLang(m.handlers.Genres))
	httpx.Raw(rt.User, huma.Operation{OperationID: "getHome", Method: http.MethodGet, Path: "/home", Tags: tags}, withLang(m.handlers.Home))
	httpx.Raw(rt.User, huma.Operation{OperationID: "browseTitles", Method: http.MethodGet, Path: "/titles", Tags: tags}, withLang(m.handlers.Browse))
	httpx.Raw(rt.User, huma.Operation{OperationID: "getTitle", Method: http.MethodGet, Path: "/titles/{slug}", Tags: tags}, withLang(m.handlers.Title))
	httpx.Raw(rt.User, huma.Operation{OperationID: "search", Method: http.MethodGet, Path: "/search", Tags: tags}, withLang(m.handlers.Search))

	httpx.Raw(rt.User, huma.Operation{OperationID: "saveProgress", Method: http.MethodPut, Path: "/progress", Tags: tags}, m.progress.Put)
	// sendBeacon can only POST
	httpx.Raw(rt.User, huma.Operation{OperationID: "saveProgressBeacon", Method: http.MethodPost, Path: "/progress", Tags: tags}, m.progress.Put)
	httpx.Raw(rt.User, huma.Operation{OperationID: "listContinueWatching", Method: http.MethodGet, Path: "/me/continue-watching", Tags: tags}, withLang(m.progress.ContinueWatching))
	httpx.Raw(rt.User, huma.Operation{OperationID: "listWatchlist", Method: http.MethodGet, Path: "/me/watchlist", Tags: tags}, withLang(m.progress.WatchlistGet))
	httpx.Raw(rt.User, huma.Operation{OperationID: "addToWatchlist", Method: http.MethodPut, Path: "/me/watchlist/{titleId}", Tags: tags}, m.progress.WatchlistPut)
	httpx.Raw(rt.User, huma.Operation{OperationID: "removeFromWatchlist", Method: http.MethodDelete, Path: "/me/watchlist/{titleId}", Tags: tags}, m.progress.WatchlistDelete)

	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminListLibrary", Method: http.MethodGet, Path: "/library", Tags: tags}, m.admin.Library)
	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminCreateTitle", Method: http.MethodPost, Path: "/titles", Tags: tags}, m.admin.Create)
	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminBulkTitles", Method: http.MethodPost, Path: "/titles/bulk", Tags: tags}, m.admin.Bulk)
	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminGetTitle", Method: http.MethodGet, Path: "/titles/{id}", Tags: tags}, m.admin.Get)
	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminGetTitleStorage", Method: http.MethodGet, Path: "/titles/{id}/storage", Tags: tags}, m.admin.Storage)
	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminUpdateTitle", Method: http.MethodPatch, Path: "/titles/{id}", Tags: tags}, m.admin.Update)
	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminSetTitleTranslation", Method: http.MethodPatch, Path: "/titles/{id}/translations/{lang}", Tags: tags}, m.admin.SetTranslation)
	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminDeleteTitleLanguage", Method: http.MethodDelete, Path: "/titles/{id}/languages/{lang}", Tags: tags}, m.admin.DeleteLanguage)
	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminDeleteTitle", Method: http.MethodDelete, Path: "/titles/{id}", Tags: tags}, m.admin.Delete)
	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminCreateSeason", Method: http.MethodPost, Path: "/titles/{id}/seasons", Tags: tags}, m.admin.CreateSeason)
	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminDeleteSeason", Method: http.MethodDelete, Path: "/seasons/{id}", Tags: tags}, m.admin.DeleteSeason)
	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminCreateEpisode", Method: http.MethodPost, Path: "/seasons/{id}/episodes", Tags: tags}, m.admin.CreateEpisode)
	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminUpdateEpisode", Method: http.MethodPatch, Path: "/episodes/{id}", Tags: tags}, m.admin.UpdateEpisode)
	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminGetEpisodeTranslations", Method: http.MethodGet, Path: "/episodes/{id}/translations", Tags: tags}, m.admin.EpisodeTranslations)
	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminSetEpisodeTranslation", Method: http.MethodPatch, Path: "/episodes/{id}/translations/{lang}", Tags: tags}, m.admin.SetEpisodeTranslation)
	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminDeleteEpisode", Method: http.MethodDelete, Path: "/episodes/{id}", Tags: tags}, m.admin.DeleteEpisode)
}
