package system

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"couchverse/internal/feature/jobs"
	"couchverse/internal/flags"
	"couchverse/internal/httpx"
	"couchverse/internal/settings"
)

// Module bundles theme, settings, storage and host-stats handlers.
type Module struct {
	theme    *Theme
	settings *AdminSettings
	storage  *AdminStorage
	stats    *SysStats
	live     *Live
	flags    *settings.Store
}

func NewModule(st *Store, set *settings.Store, jb *jobs.Store, dataDir string, couch CouchPresence, transcodes TranscodePresence) *Module {
	return &Module{
		theme:    NewTheme(set),
		settings: NewAdminSettings(set),
		storage:  NewAdminStorage(st, jb, dataDir),
		stats:    NewSysStats(),
		live:     NewLive(st, couch, transcodes),
		flags:    set,
	}
}

func (m *Module) Register(rt httpx.Routes) {
	tags := []string{"system"}
	// public: the accent theme applies on the login screen too
	httpx.Raw(rt.Public, huma.Operation{OperationID: "getTheme", Method: http.MethodGet, Path: "/theme", Tags: tags}, m.theme.Get)
	httpx.Raw(rt.User, huma.Operation{OperationID: "getFeatures", Method: http.MethodGet, Path: "/features", Tags: tags}, m.Features)

	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminGetSettings", Method: http.MethodGet, Path: "/settings", Tags: tags}, m.settings.Get)
	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminUpdateSettings", Method: http.MethodPut, Path: "/settings", Tags: tags}, m.settings.Put)
	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminGetStorage", Method: http.MethodGet, Path: "/storage", Tags: tags}, m.storage.Get)
	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminGetOverview", Method: http.MethodGet, Path: "/overview", Tags: tags}, m.storage.Overview)
	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminGetSystemStats", Method: http.MethodGet, Path: "/system", Tags: tags}, m.stats.Get)
	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminGetLive", Method: http.MethodGet, Path: "/live", Tags: tags}, m.live.Get)
	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminGetHomeRows", Method: http.MethodGet, Path: "/home-rows", Tags: tags}, m.storage.HomeRowsGet)
	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminUpdateHomeRows", Method: http.MethodPut, Path: "/home-rows", Tags: tags}, m.storage.HomeRowsPut)
}

// Features reports the admin-toggleable feature flags to the app shell.
func (m *Module) Features(w http.ResponseWriter, r *http.Request) {
	httpx.JSON(w, http.StatusOK, flags.Load(r.Context(), m.flags))
}
