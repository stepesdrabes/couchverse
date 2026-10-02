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
	CouchEnabled    bool `json:"couchEnabled"`
	RankingsEnabled bool `json:"rankingsEnabled"`
}

// Load reads the "features" settings key; absent or malformed means everything
// is enabled (backwards compatible).
func Load(ctx context.Context, st *settings.Store) Flags {
	f := Flags{CouchEnabled: true, RankingsEnabled: true}
	raw, err := st.Get(ctx, "features")
	if err == nil && raw != nil {
		_ = json.Unmarshal(raw, &f)
	}
	return f
}

// CouchOn is an httpx.Guard check that hides couch-session routes entirely while
// the feature is disabled.
func CouchOn(st *settings.Store) func(context.Context) error {
	return func(ctx context.Context) error {
		if !Load(ctx, st).CouchEnabled {
			return httpx.Fail(http.StatusNotFound, "feature_disabled", "couch sessions are disabled")
		}
		return nil
	}
}

// RankingsOn is an httpx.Guard check that hides profile and leaderboard routes
// entirely while the feature is disabled. The couch counters keep accruing
// regardless, so turning it back on does not present an empty history.
func RankingsOn(st *settings.Store) func(context.Context) error {
	return func(ctx context.Context) error {
		if !Load(ctx, st).RankingsEnabled {
			return httpx.Fail(http.StatusNotFound, "feature_disabled", "rankings are disabled")
		}
		return nil
	}
}
