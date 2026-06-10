package api

import (
	"net/http"

	"couchverse/internal/auth"
	"couchverse/internal/httpx"
	"couchverse/internal/store"
)

type AdminUsers struct {
	store *store.Store
}

func NewAdminUsers(st *store.Store) *AdminUsers {
	return &AdminUsers{store: st}
}

func (h *AdminUsers) List(w http.ResponseWriter, r *http.Request) {
	users, err := h.store.ListUsers(r.Context())
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, users)
}

func (h *AdminUsers) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username    string `json:"username"`
		DisplayName string `json:"displayName"`
		Password    string `json:"password"`
		Role        string `json:"role"`
	}
	if err := httpx.Decode(r, &req); err != nil {
		httpx.BadRequest(w, "invalid request body")
		return
	}
	if req.Username == "" || len(req.Password) < 4 {
		httpx.BadRequest(w, "username and a password of at least 4 characters are required")
		return
	}
	if req.Role != "admin" {
		req.Role = "member"
	}
	if req.DisplayName == "" {
		req.DisplayName = req.Username
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	user, err := h.store.CreateUser(r.Context(), req.Username, req.DisplayName, hash, req.Role)
	if err != nil {
		httpx.Error(w, http.StatusConflict, "conflict", "username already exists")
		return
	}
	httpx.JSON(w, http.StatusCreated, user)
}

func (h *AdminUsers) Update(w http.ResponseWriter, r *http.Request) {
	id := httpx.ID(r, "id")
	self := auth.UserFrom(r.Context())

	var req struct {
		DisplayName *string `json:"displayName"`
		Role        *string `json:"role"`
		Disabled    *bool   `json:"disabled"`
		Password    *string `json:"password"`
	}
	if err := httpx.Decode(r, &req); err != nil {
		httpx.BadRequest(w, "invalid request body")
		return
	}

	if id == self.ID && ((req.Disabled != nil && *req.Disabled) || (req.Role != nil && *req.Role != "admin")) {
		httpx.BadRequest(w, "you cannot disable or demote your own account")
		return
	}
	if req.Role != nil && *req.Role != "admin" && *req.Role != "member" {
		httpx.BadRequest(w, "invalid role")
		return
	}

	up := store.UserUpdate{DisplayName: req.DisplayName, Role: req.Role, Disabled: req.Disabled}
	if req.Password != nil {
		if len(*req.Password) < 4 {
			httpx.BadRequest(w, "password must be at least 4 characters")
			return
		}
		hash, err := auth.HashPassword(*req.Password)
		if err != nil {
			httpx.Internal(w, err)
			return
		}
		up.PasswordHash = &hash
	}

	user, err := h.store.UpdateUser(r.Context(), id, up)
	if err != nil {
		httpx.StoreErr(w, err)
		return
	}
	if req.Disabled != nil && *req.Disabled {
		if err := h.store.DeleteUserSessions(r.Context(), id); err != nil {
			httpx.Internal(w, err)
			return
		}
	}
	httpx.JSON(w, http.StatusOK, user)
}

func (h *AdminUsers) Delete(w http.ResponseWriter, r *http.Request) {
	id := httpx.ID(r, "id")
	if id == auth.UserFrom(r.Context()).ID {
		httpx.BadRequest(w, "you cannot delete your own account")
		return
	}
	if err := h.store.DeleteUser(r.Context(), id); err != nil {
		httpx.StoreErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}
