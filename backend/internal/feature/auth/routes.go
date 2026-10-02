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

func (m *Module) Register(rt httpx.Routes) {
	tags := []string{"auth"}
	httpx.Raw(rt.Public, huma.Operation{OperationID: "login", Method: http.MethodPost, Path: "/auth/login", Tags: tags}, m.handlers.Login)
	httpx.Raw(rt.Public, huma.Operation{OperationID: "logout", Method: http.MethodPost, Path: "/auth/logout", Tags: tags}, m.handlers.Logout)

	httpx.Raw(rt.User, huma.Operation{OperationID: "getMe", Method: http.MethodGet, Path: "/auth/me", Tags: tags}, m.handlers.Me)
	httpx.Raw(rt.User, huma.Operation{OperationID: "updateProfile", Method: http.MethodPatch, Path: "/me/profile", Tags: tags}, m.profile.Update)
	httpx.Raw(rt.User, huma.Operation{OperationID: "changePassword", Method: http.MethodPatch, Path: "/me/password", Tags: tags}, m.profile.ChangePassword)
	httpx.Raw(rt.User, huma.Operation{OperationID: "getPreferences", Method: http.MethodGet, Path: "/me/preferences", Tags: tags}, m.profile.Preferences)
	httpx.Raw(rt.User, huma.Operation{OperationID: "updatePreferences", Method: http.MethodPut, Path: "/me/preferences", Tags: tags}, m.profile.UpdatePreferences)
	httpx.Raw(rt.User, huma.Operation{OperationID: "uploadAvatar", Method: http.MethodPost, Path: "/me/avatar", Tags: tags}, m.profile.SetAvatar)
	httpx.Raw(rt.User, huma.Operation{OperationID: "deleteAvatar", Method: http.MethodDelete, Path: "/me/avatar", Tags: tags}, m.profile.DeleteAvatar)
	httpx.Raw(rt.User, huma.Operation{OperationID: "uploadBanner", Method: http.MethodPost, Path: "/me/banner", Tags: tags}, m.profile.SetBanner)
	httpx.Raw(rt.User, huma.Operation{OperationID: "deleteBanner", Method: http.MethodDelete, Path: "/me/banner", Tags: tags}, m.profile.DeleteBanner)

	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminListUsers", Method: http.MethodGet, Path: "/users", Tags: tags}, m.admin.List)
	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminCreateUser", Method: http.MethodPost, Path: "/users", Tags: tags}, m.admin.Create)
	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminUpdateUser", Method: http.MethodPatch, Path: "/users/{id}", Tags: tags}, m.admin.Update)
	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminDeleteUser", Method: http.MethodDelete, Path: "/users/{id}", Tags: tags}, m.admin.Delete)
}
