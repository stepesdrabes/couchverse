package ranks

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"couchverse/internal/flags"
	"couchverse/internal/httpx"
	"couchverse/internal/settings"
)

// Module mounts the progression routes. The whole group is hidden when the
// rankingsEnabled flag is off, mirroring couch.
type Module struct {
	handlers *Handlers
	admin    *AdminRanks
	settings *settings.Store
}

func NewModule(st *Store, set *settings.Store) *Module {
	return &Module{handlers: NewHandlers(st, set), admin: NewAdminRanks(st, set), settings: set}
}

const tag httpx.Tag = "ranks"

// Register adds the progression routes. The viewer group is hidden while the
// rankingsEnabled flag is off; the admin is deliberately not gated, since an
// admin has to be able to inspect and retune ranks in order to turn them back on.
func (m *Module) Register(rt httpx.Routes) {
	api := huma.NewGroup(rt.User)
	api.UseMiddleware(httpx.Guard(api, flags.RankingsOn(m.settings)))
	// localized so a profile's title names and favourite genre read in the
	// visitor's language
	huma.Register(api, httpx.Localized(tag.Op("getMyStats", http.MethodGet, "/me/stats")), m.handlers.MyStats)
	huma.Register(api, tag.Op("checkAchievements", http.MethodPost, "/me/achievements/check"), m.handlers.Check)
	huma.Register(api, httpx.Localized(tag.Op("getProfile", http.MethodGet, "/users/{username}/profile")), m.handlers.PublicProfile)
	huma.Register(api, tag.Op("getLeaderboard", http.MethodGet, "/leaderboard"), m.handlers.Leaderboard)

	huma.Register(rt.Admin, tag.Op("adminGetRanks", http.MethodGet, "/ranks"), m.admin.Overview)
	huma.Register(rt.Admin, tag.Op("adminUpdateRanksConfig", http.MethodPut, "/ranks/config"), m.admin.PutConfig)
}
