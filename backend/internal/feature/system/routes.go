package system

import (
	"context"
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

const tag httpx.Tag = "system"

func (m *Module) Register(rt httpx.Routes) {
	// public: the accent theme applies on the login screen too
	huma.Register(rt.Public, tag.Op("getServer", http.MethodGet, "/server"), m.theme.Server)
	huma.Register(rt.Public, tag.Op("getTheme", http.MethodGet, "/theme"), m.theme.Get)
	huma.Register(rt.User, tag.Op("getFeatures", http.MethodGet, "/features"), m.Features)

	huma.Register(rt.Admin, tag.Op("adminGetSettings", http.MethodGet, "/settings"), m.settings.Get)
	huma.Register(rt.Admin, tag.Op("adminUpdateSettings", http.MethodPut, "/settings"), m.settings.Put)
	huma.Register(rt.Admin, tag.Op("adminGetStorage", http.MethodGet, "/storage"), m.storage.Get)
	huma.Register(rt.Admin, tag.Op("adminGetOverview", http.MethodGet, "/overview"), m.storage.Overview)
	huma.Register(rt.Admin, tag.Op("adminGetSystemStats", http.MethodGet, "/system"), m.stats.Get)
	huma.Register(rt.Admin, tag.Op("adminGetLive", http.MethodGet, "/live"), m.live.Get)
	huma.Register(rt.Admin, tag.Op("adminGetHomeRows", http.MethodGet, "/home-rows"), m.storage.HomeRowsGet)
	huma.Register(rt.Admin, tag.Op("adminUpdateHomeRows", http.MethodPut, "/home-rows"), m.storage.HomeRowsPut)
}

type featuresOutput struct{ Body FeatureFlags }

// Features reports the admin-toggleable feature flags to the app shell.
func (m *Module) Features(ctx context.Context, _ *struct{}) (*featuresOutput, error) {
	return &featuresOutput{Body: FeatureFlags(flags.Load(ctx, m.flags))}, nil
}
