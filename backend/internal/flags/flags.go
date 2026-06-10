// Package flags holds admin-toggleable feature flags stored in settings.
package flags

import (
	"context"
	"encoding/json"
	"net/http"

	"couchverse/internal/httpx"
	"couchverse/internal/store"
)

type Flags struct {
	MusicEnabled bool `json:"musicEnabled"`
}

// Load reads the "features" settings key; absent or malformed means everything
// is enabled (backwards compatible).
func Load(ctx context.Context, st *store.Store) Flags {
	f := Flags{MusicEnabled: true}
	raw, err := st.Setting(ctx, "features")
	if err == nil && raw != nil {
		_ = json.Unmarshal(raw, &f)
	}
	return f
}

// RequireMusic hides music routes entirely while the feature is disabled.
func RequireMusic(st *store.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !Load(r.Context(), st).MusicEnabled {
				httpx.Error(w, http.StatusNotFound, "feature_disabled", "music is disabled")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
