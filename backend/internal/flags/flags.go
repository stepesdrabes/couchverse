// Package flags holds admin-toggleable feature flags stored in settings.
package flags

import (
	"context"
	"encoding/json"
	"net/http"

	"couchverse/internal/httpx"
	"couchverse/internal/settings"
)

type Flags struct {
	MusicEnabled bool `json:"musicEnabled"`
	CouchEnabled bool `json:"couchEnabled"`
}

// Load reads the "features" settings key; absent or malformed means everything
// is enabled (backwards compatible).
func Load(ctx context.Context, st *settings.Store) Flags {
	f := Flags{MusicEnabled: true, CouchEnabled: true}
	raw, err := st.Get(ctx, "features")
	if err == nil && raw != nil {
		_ = json.Unmarshal(raw, &f)
	}
	return f
}

// RequireMusic hides music routes entirely while the feature is disabled.
func RequireMusic(st *settings.Store) func(http.Handler) http.Handler {
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

// RequireCouch hides couch-session routes entirely while the feature is disabled.
func RequireCouch(st *settings.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !Load(r.Context(), st).CouchEnabled {
				httpx.Error(w, http.StatusNotFound, "feature_disabled", "couch sessions are disabled")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
