package auth

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"couchverse/internal/config"
	"couchverse/internal/feature/artwork"
	"couchverse/internal/httpx"
)

// Module bundles the auth, profile and user-admin handlers.
type Module struct {
	handlers *Handlers
	profile  *Profile
	admin    *AdminUsers
}

func NewModule(st *Store, cfg config.Config, art *artwork.Service) *Module {
	return &Module{
		handlers: NewHandlers(st, cfg),
		profile:  NewProfile(st, art),
		admin:    NewAdminUsers(st),
	}
}

const tag httpx.Tag = "auth"

func (m *Module) Register(rt httpx.Routes) {
	huma.Register(rt.Public, tag.Op("login", http.MethodPost, "/auth/login"), m.handlers.Login)
	huma.Register(rt.Public, tag.NoContent("logout", http.MethodPost, "/auth/logout"), m.handlers.Logout)
	huma.Register(rt.Public, tag.Op("signInDevice", http.MethodPost, "/auth/token"), m.handlers.Token)
	huma.Register(rt.Public, tag.Created("startPairing", http.MethodPost, "/auth/pairings"), m.handlers.StartPairing)
	huma.Register(rt.Public, tag.Op("pollPairing", http.MethodPost, "/auth/pairings/poll"), m.handlers.PollPairing)
	huma.Register(rt.Public, tag.Op("connectDevice", http.MethodPost, "/auth/connect"), m.handlers.Connect)

	huma.Register(rt.User, tag.Op("getMe", http.MethodGet, "/auth/me"), m.handlers.Me)
	huma.Register(rt.User, tag.Op("updateProfile", http.MethodPatch, "/me/profile"), m.profile.Update)
	huma.Register(rt.User, tag.NoContent("changePassword", http.MethodPatch, "/me/password"), m.profile.ChangePassword)
	huma.Register(rt.User, tag.Op("getPreferences", http.MethodGet, "/me/preferences"), m.profile.Preferences)
	huma.Register(rt.User, tag.Op("updatePreferences", http.MethodPut, "/me/preferences"), m.profile.UpdatePreferences)
	huma.Register(rt.User, tag.Op("uploadAvatar", http.MethodPost, "/me/avatar"), m.profile.SetAvatar)
	huma.Register(rt.User, tag.Op("deleteAvatar", http.MethodDelete, "/me/avatar"), m.profile.DeleteAvatar)
	huma.Register(rt.User, tag.Op("uploadBanner", http.MethodPost, "/me/banner"), m.profile.SetBanner)
	huma.Register(rt.User, tag.Op("deleteBanner", http.MethodDelete, "/me/banner"), m.profile.DeleteBanner)
	huma.Register(rt.User, tag.Op("listDevices", http.MethodGet, "/me/devices"), m.profile.Devices)
	huma.Register(rt.User, tag.NoContent("revokeDevice", http.MethodDelete, "/me/devices/{id}"), m.profile.RevokeDevice)
	huma.Register(rt.User, tag.Op("getPairingRequest", http.MethodGet, "/me/pairings/{code}"), m.profile.PairingRequest)
	huma.Register(rt.User, tag.NoContent("approvePairing", http.MethodPost, "/me/pairings/{code}/approve"), m.profile.ApprovePairing)
	huma.Register(rt.User, tag.NoContent("denyPairing", http.MethodPost, "/me/pairings/{code}/deny"), m.profile.DenyPairing)
	huma.Register(rt.User, tag.Created("createConnectCode", http.MethodPost, "/me/connect-codes"), m.profile.CreateConnectCode)

	huma.Register(rt.Admin, tag.Op("adminListUsers", http.MethodGet, "/users"), m.admin.List)
	huma.Register(rt.Admin, tag.Created("adminCreateUser", http.MethodPost, "/users"), m.admin.Create)
	huma.Register(rt.Admin, tag.Op("adminUpdateUser", http.MethodPatch, "/users/{id}"), m.admin.Update)
	huma.Register(rt.Admin, tag.NoContent("adminDeleteUser", http.MethodDelete, "/users/{id}"), m.admin.Delete)
}
