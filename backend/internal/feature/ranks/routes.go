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

// Register adds the progression routes. The viewer group is hidden while the
// rankingsEnabled flag is off; the admin is deliberately not gated, since an
// admin has to be able to inspect and retune ranks in order to turn them back on.
func (m *Module) Register(rt httpx.Routes) {
	tags := []string{"ranks"}
	api := huma.NewGroup(rt.User)
	api.UseMiddleware(httpx.Guard(api, flags.RankingsOn(m.settings)))
	httpx.Raw(api, huma.Operation{OperationID: "getMyStats", Method: http.MethodGet, Path: "/me/stats", Tags: tags}, withLang(m.handlers.MyStats))
	httpx.Raw(api, huma.Operation{OperationID: "checkAchievements", Method: http.MethodPost, Path: "/me/achievements/check", Tags: tags}, m.handlers.Check)
	httpx.Raw(api, huma.Operation{OperationID: "getProfile", Method: http.MethodGet, Path: "/users/{username}/profile", Tags: tags}, withLang(m.handlers.PublicProfile))
	httpx.Raw(api, huma.Operation{OperationID: "getLeaderboard", Method: http.MethodGet, Path: "/leaderboard", Tags: tags}, m.handlers.Leaderboard)

	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminGetRanks", Method: http.MethodGet, Path: "/ranks", Tags: tags}, m.admin.Overview)
	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminUpdateRanksConfig", Method: http.MethodPut, Path: "/ranks/config", Tags: tags}, m.admin.PutConfig)
}

// withLang threads the ?lang= display language onto the request context so a
// profile's title names and favourite genre read in the visitor's language.
func withLang(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h(w, r.WithContext(httpx.WithLang(r.Context(), httpx.Lang(r))))
	}
}
