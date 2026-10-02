package couch

import (
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"couchverse/internal/flags"
	"couchverse/internal/httpx"
	"couchverse/internal/settings"
)

// Module mounts the couch HTTP + WebSocket routes.
type Module struct {
	hub      *Hub
	handlers *Handlers
	settings *settings.Store
}

func NewModule(hub *Hub, set *settings.Store) *Module {
	return &Module{
		hub:      hub,
		handlers: &Handlers{hub: hub, joinRate: newRateLimiter(10, time.Minute)},
		settings: set,
	}
}

const tag httpx.Tag = "couch"

// Register adds the couch routes to the public group (anonymous followers must
// reach them); each handler enforces its own auth. The whole group is hidden
// when the couchEnabled flag is off.
func (m *Module) Register(rt httpx.Routes) {
	api := huma.NewGroup(rt.Public)
	api.UseMiddleware(httpx.Guard(api, flags.CouchOn(m.settings)))

	create := tag.Created("createCouch", http.MethodPost, "/couch")
	create.Middlewares = huma.Middlewares{httpx.Guard(api, hostSignedIn)}
	create.Security = []map[string][]string{{"cookieSession": {}}, {"bearerDevice": {}}}
	huma.Register(api, create, m.handlers.Create)
	huma.Register(api, httpx.Localized(tag.Op("getCouchInfo", http.MethodGet, "/couch/{token}/info")), m.handlers.Info)
	huma.Register(api, tag.Op("joinCouch", http.MethodPost, "/couch/{token}/join"), m.handlers.Join)
	huma.Register(api, tag.NoContent("leaveCouch", http.MethodPost, "/couch/{token}/leave"), m.handlers.Leave)
	huma.Register(api, tag.NoContent("endCouch", http.MethodPost, "/couch/{token}/end"), m.handlers.End)
	huma.Register(api, httpx.Localized(tag.Op("getCouchPlayback", http.MethodGet, "/couch/{token}/playback")), m.handlers.Playback)
	httpx.Raw(api, socketOp(api), m.handlers.WS)
}

// socketOp describes the WebSocket upgrade, which stays a plain handler. The
// messages it carries are described separately from this API.
func socketOp(api huma.API) huma.Operation {
	op := tag.Op("couchSocket", http.MethodGet, "/couch/{token}/ws")
	op.Summary = "Open a couch session's WebSocket"
	op.Description = "Upgrades to the session's sync channel, authenticated by the participant token " +
		"from createCouch or joinCouch: the couch cookie, or the X-Couch-Token header for clients " +
		"that joined with delivery=body (401 no_couch_session without one). Cross-origin browser " +
		"upgrades are refused."
	op.Parameters = []*huma.Param{
		{Name: "token", In: "path", Required: true, Description: "The session's share code.",
			Schema: &huma.Schema{Type: huma.TypeString}},
		{Name: CouchCookie, In: "cookie", Description: "The participant cookie set by createCouch or joinCouch.",
			Schema: &huma.Schema{Type: huma.TypeString}},
		{Name: CouchTokenHeader, In: "header", Description: "The participant token from createCouch or joinCouch with delivery=body.",
			Schema: &huma.Schema{Type: huma.TypeString}},
	}
	op.Responses = map[string]*huma.Response{
		"101":     {Description: "Switched to the WebSocket protocol."},
		"default": httpx.ErrorResponse(api),
	}
	return op
}
