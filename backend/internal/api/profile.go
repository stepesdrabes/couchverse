package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"couchverse/internal/artwork"
	"couchverse/internal/auth"
	"couchverse/internal/httpx"
	"couchverse/internal/store"
)

type Profile struct {
	store   *store.Store
	artwork *artwork.Service
}

func NewProfile(st *store.Store, art *artwork.Service) *Profile {
	return &Profile{store: st, artwork: art}
}

// Update lets users change their own display name.
func (h *Profile) Update(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DisplayName string `json:"displayName"`
	}
	if err := httpx.Decode(r, &req); err != nil || req.DisplayName == "" {
		httpx.BadRequest(w, "displayName is required")
		return
	}
	user, err := h.store.UpdateUser(r.Context(), auth.UserFrom(r.Context()).ID,
		store.UserUpdate{DisplayName: &req.DisplayName})
	if err != nil {
		httpx.StoreErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, user)
}

// Preferences returns the caller's settings blob (subtitle styling, etc.).
func (h *Profile) Preferences(w http.ResponseWriter, r *http.Request) {
	prefs, err := h.store.UserPreferences(r.Context(), auth.UserFrom(r.Context()).ID)
	if err != nil {
		httpx.StoreErr(w, err)
		return
	}
	if prefs == nil {
		prefs = json.RawMessage("{}")
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(prefs)
}

// UpdatePreferences shallow-merges the posted top-level keys.
func (h *Profile) UpdatePreferences(w http.ResponseWriter, r *http.Request) {
	var patch map[string]json.RawMessage
	if err := httpx.Decode(r, &patch); err != nil {
		httpx.BadRequest(w, "invalid request body")
		return
	}
	prefs, err := h.store.MergeUserPreferences(r.Context(), auth.UserFrom(r.Context()).ID, patch)
	if err != nil {
		httpx.StoreErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(prefs)
}

// SetAvatar accepts a multipart image and stores it as the user's avatar.
func (h *Profile) SetAvatar(w http.ResponseWriter, r *http.Request) {
	self := auth.UserFrom(r.Context())
	if err := r.ParseMultipartForm(16 << 20); err != nil {
		httpx.BadRequest(w, "invalid multipart form")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		httpx.BadRequest(w, "file field is required")
		return
	}
	defer file.Close()

	if _, err := h.artwork.Save(r.Context(), "user", strconv.FormatInt(self.ID, 10), "avatar", header.Filename, file); err != nil {
		httpx.Error(w, http.StatusBadRequest, "avatar_failed", err.Error())
		return
	}
	user, err := h.store.UserByID(r.Context(), self.ID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, user)
}

// DeleteAvatar removes the user's profile picture.
func (h *Profile) DeleteAvatar(w http.ResponseWriter, r *http.Request) {
	self := auth.UserFrom(r.Context())
	if self.AvatarID != nil {
		if err := h.artwork.Delete(r.Context(), *self.AvatarID); err != nil {
			httpx.StoreErr(w, err)
			return
		}
	}
	user, err := h.store.UserByID(r.Context(), self.ID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, user)
}
