package system

import (
	"net/http"

	"github.com/go-chi/chi/v5"

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

// MountPublic mounts routes that work without a session - the accent theme
// applies on the login screen too.
func (m *Module) MountPublic(r chi.Router) {
	r.Get("/theme", m.theme.Get)
}

// Favicon serves the accent-tinted app icon (registered at the root, not /api).
func (m *Module) Favicon(w http.ResponseWriter, r *http.Request) {
	m.theme.Favicon(w, r)
}

func (m *Module) MountUser(r chi.Router) {
	r.Get("/features", m.Features)
}

func (m *Module) MountAdmin(r chi.Router) {
	r.Get("/settings", m.settings.Get)
	r.Put("/settings", m.settings.Put)

	r.Get("/storage", m.storage.Get)
	r.Get("/overview", m.storage.Overview)
	r.Get("/system", m.stats.Get)
	r.Get("/live", m.live.Get)
	r.Get("/home-rows", m.storage.HomeRowsGet)
	r.Put("/home-rows", m.storage.HomeRowsPut)
}

// Features reports the admin-toggleable feature flags to the app shell.
func (m *Module) Features(w http.ResponseWriter, r *http.Request) {
	httpx.JSON(w, http.StatusOK, flags.Load(r.Context(), m.flags))
}
