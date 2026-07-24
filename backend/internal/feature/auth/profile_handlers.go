package auth

import (
	"encoding/json"
	"net/http"
	"strconv"

	"couchverse/internal/feature/artwork"
	"couchverse/internal/httpx"
)

type Profile struct {
	store   *Store
	artwork *artwork.Service
}

func NewProfile(st *Store, art *artwork.Service) *Profile {
	return &Profile{store: st, artwork: art}
}

// MaxBioLength bounds the public-profile bio. Markdown is stored as authored and
// rendered client-side with raw HTML disabled.
const MaxBioLength = 2000

// Update lets users change their own display name and bio. Bio is a pointer so
// clearing it ("") is distinguishable from not touching it.
func (h *Profile) Update(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DisplayName string  `json:"displayName"`
		Bio         *string `json:"bio"`
	}
	if err := httpx.Decode(r, &req); err != nil || req.DisplayName == "" {
		httpx.BadRequest(w, "displayName is required")
		return
	}
	if req.Bio != nil && len(*req.Bio) > MaxBioLength {
		httpx.BadRequest(w, "bio is too long")
		return
	}
	user, err := h.store.UpdateUser(r.Context(), UserFrom(r.Context()).ID,
		UserUpdate{DisplayName: &req.DisplayName, Bio: req.Bio})
	if err != nil {
		httpx.StoreErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, user)
}

// ChangePassword lets a signed-in user set a new password after re-entering the
// current one. The session is kept (no forced re-login).
func (h *Profile) ChangePassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if err := httpx.Decode(r, &req); err != nil {
		httpx.BadRequest(w, "invalid request body")
		return
	}
	if len(req.NewPassword) < 8 {
		httpx.BadRequest(w, "password must be at least 8 characters")
		return
	}
	self := UserFrom(r.Context())
	ok, err := VerifyPassword(req.CurrentPassword, self.PasswordHash)
	if err != nil || !ok {
		httpx.Error(w, http.StatusBadRequest, "invalid_password", "current password is incorrect")
		return
	}
	hash, err := HashPassword(req.NewPassword)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if _, err := h.store.UpdateUser(r.Context(), self.ID, UserUpdate{PasswordHash: &hash}); err != nil {
		httpx.StoreErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Preferences returns the caller's settings blob (subtitle styling, etc.).
func (h *Profile) Preferences(w http.ResponseWriter, r *http.Request) {
	prefs, err := h.store.UserPreferences(r.Context(), UserFrom(r.Context()).ID)
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
	prefs, err := h.store.MergeUserPreferences(r.Context(), UserFrom(r.Context()).ID, patch)
	if err != nil {
		httpx.StoreErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(prefs)
}

// SetAvatar accepts a multipart image and stores it as the user's avatar.
func (h *Profile) SetAvatar(w http.ResponseWriter, r *http.Request) {
	h.setImage(w, r, "avatar")
}

// DeleteAvatar removes the user's profile picture.
func (h *Profile) DeleteAvatar(w http.ResponseWriter, r *http.Request) {
	h.deleteImage(w, r, func(u *User) *string { return u.AvatarID })
}

// SetBanner accepts a multipart image and stores it as the profile banner. It is
// an ordinary artwork row, so it gets the same resizing, caching and accent
// extraction as posters do - the profile hero is tinted from it.
func (h *Profile) SetBanner(w http.ResponseWriter, r *http.Request) {
	h.setImage(w, r, "banner")
}

// DeleteBanner removes the profile banner.
func (h *Profile) DeleteBanner(w http.ResponseWriter, r *http.Request) {
	h.deleteImage(w, r, func(u *User) *string { return u.BannerID })
}

func (h *Profile) setImage(w http.ResponseWriter, r *http.Request, kind string) {
	self := UserFrom(r.Context())
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

	owner := strconv.FormatInt(self.ID, 10)
	if _, err := h.artwork.Save(r.Context(), "user", owner, kind, header.Filename, file); err != nil {
		httpx.Error(w, http.StatusBadRequest, kind+"_failed", err.Error())
		return
	}
	h.respondSelf(w, r, self.ID)
}

func (h *Profile) deleteImage(w http.ResponseWriter, r *http.Request, pick func(*User) *string) {
	self := UserFrom(r.Context())
	if id := pick(self); id != nil {
		if err := h.artwork.Delete(r.Context(), *id); err != nil {
			httpx.StoreErr(w, err)
			return
		}
	}
	h.respondSelf(w, r, self.ID)
}

func (h *Profile) respondSelf(w http.ResponseWriter, r *http.Request, id int64) {
	user, err := h.store.UserByID(r.Context(), id)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, user)
}
