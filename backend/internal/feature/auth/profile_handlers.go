package auth

import (
	"context"
	"net/http"
	"strconv"

	"github.com/danielgtaylor/huma/v2"

	"couchverse/internal/feature/artwork"
	"couchverse/internal/grant"
	"couchverse/internal/httpx"
)

type Profile struct {
	store   *Store
	artwork *artwork.Service
	grants  *grant.Signer
}

func NewProfile(st *Store, art *artwork.Service, grants *grant.Signer) *Profile {
	return &Profile{store: st, artwork: art, grants: grants}
}

// ProfileUpdate is what users may change about themselves. Bio is a pointer so
// clearing it ("") is distinguishable from not touching it; it is markdown,
// stored as authored and rendered client-side with raw HTML disabled.
type ProfileUpdate struct {
	DisplayName string  `json:"displayName" minLength:"1"`
	Bio         *string `json:"bio" required:"false" maxLength:"2000" doc:"Markdown; absent keeps the current bio."`
}

type updateProfileInput struct{ Body ProfileUpdate }

func (h *Profile) Update(ctx context.Context, in *updateProfileInput) (*userOutput, error) {
	user, err := h.store.UpdateUser(ctx, UserFrom(ctx).ID,
		UserUpdate{DisplayName: &in.Body.DisplayName, Bio: in.Body.Bio})
	if err != nil {
		return nil, err
	}
	return &userOutput{Body: user}, nil
}

type PasswordChange struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword" minLength:"8"`
}

type changePasswordInput struct{ Body PasswordChange }

// ChangePassword lets a signed-in user set a new password after re-entering the
// current one. The session is kept (no forced re-login).
func (h *Profile) ChangePassword(ctx context.Context, in *changePasswordInput) (*struct{}, error) {
	self := UserFrom(ctx)
	ok, err := VerifyPassword(in.Body.CurrentPassword, self.PasswordHash)
	if err != nil || !ok {
		return nil, httpx.Fail(http.StatusBadRequest, "invalid_password", "current password is incorrect")
	}
	hash, err := HashPassword(in.Body.NewPassword)
	if err != nil {
		return nil, err
	}
	if _, err := h.store.UpdateUser(ctx, self.ID, UserUpdate{PasswordHash: &hash}); err != nil {
		return nil, err
	}
	return nil, nil
}

type preferencesOutput struct{ Body *Preferences }

// Preferences returns the caller's client settings (subtitle styling, etc.).
func (h *Profile) Preferences(ctx context.Context, _ *struct{}) (*preferencesOutput, error) {
	prefs, err := h.store.UserPreferences(ctx, UserFrom(ctx).ID)
	if err != nil {
		return nil, err
	}
	return &preferencesOutput{Body: prefs}, nil
}

type updatePreferencesInput struct {
	// Body is a patch: absent keys keep their stored value.
	Body Preferences
}

// UpdatePreferences merges the posted keys and returns the result.
func (h *Profile) UpdatePreferences(ctx context.Context, in *updatePreferencesInput) (*preferencesOutput, error) {
	prefs, err := h.store.MergeUserPreferences(ctx, UserFrom(ctx).ID, in.Body)
	if err != nil {
		return nil, err
	}
	return &preferencesOutput{Body: prefs}, nil
}

type imageForm struct {
	File huma.FormFile `form:"file" required:"true" doc:"JPEG, PNG or WebP image, typed by its file name extension."`
}

type imageUploadInput struct {
	RawBody huma.MultipartFormFiles[imageForm]
}

// SetAvatar stores an uploaded image as the user's avatar.
func (h *Profile) SetAvatar(ctx context.Context, in *imageUploadInput) (*userOutput, error) {
	return h.setImage(ctx, in, "avatar")
}

// DeleteAvatar removes the user's profile picture.
func (h *Profile) DeleteAvatar(ctx context.Context, _ *struct{}) (*userOutput, error) {
	return h.deleteImage(ctx, UserFrom(ctx).AvatarID)
}

// SetBanner stores an uploaded image as the profile banner. It is an ordinary
// artwork row, so it gets the same resizing, caching and accent extraction as
// posters do - the profile hero is tinted from it.
func (h *Profile) SetBanner(ctx context.Context, in *imageUploadInput) (*userOutput, error) {
	return h.setImage(ctx, in, "banner")
}

// DeleteBanner removes the profile banner.
func (h *Profile) DeleteBanner(ctx context.Context, _ *struct{}) (*userOutput, error) {
	return h.deleteImage(ctx, UserFrom(ctx).BannerID)
}

func (h *Profile) setImage(ctx context.Context, in *imageUploadInput, kind string) (*userOutput, error) {
	self := UserFrom(ctx)
	file := in.RawBody.Data().File
	defer file.Close()

	owner := strconv.FormatInt(self.ID, 10)
	if _, err := h.artwork.Save(ctx, "user", owner, kind, "", file.Filename, file); err != nil {
		return nil, httpx.Fail(http.StatusBadRequest, kind+"_failed", err.Error())
	}
	return h.reload(ctx, self.ID)
}

func (h *Profile) deleteImage(ctx context.Context, id *string) (*userOutput, error) {
	if id != nil {
		if err := h.artwork.Delete(ctx, *id); err != nil {
			return nil, err
		}
	}
	return h.reload(ctx, UserFrom(ctx).ID)
}

func (h *Profile) reload(ctx context.Context, id int64) (*userOutput, error) {
	user, err := h.store.UserByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &userOutput{Body: user}, nil
}

type artworkGrantOutput struct{ Body artwork.ArtworkGrant }

// ArtworkGrant issues a grant for system components that fetch artwork without
// the session (tvOS Top Shelf, AirPlay receivers).
func (p *Profile) ArtworkGrant(ctx context.Context, _ *struct{}) (*artworkGrantOutput, error) {
	return &artworkGrantOutput{Body: artwork.IssueGrant(p.grants, UserFrom(ctx).ID)}, nil
}
