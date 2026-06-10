package api

import (
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
		respondStoreErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, user)
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
			respondStoreErr(w, err)
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
