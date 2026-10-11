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
	CouchEnabled     bool `json:"couchEnabled"`
	RankingsEnabled  bool `json:"rankingsEnabled"`
	DownloadsEnabled bool `json:"downloadsEnabled"`
	CouchTvEnabled   bool `json:"couchTvEnabled,omitempty"`
}

// Load reads the "features" settings key; absent or malformed means every flag
// that predates it is enabled (backwards compatible). Couch TV is the exception:
// it stays off until an admin turns it on, since the channel needs a schedule and
// rendered breaks first.
func Load(ctx context.Context, st *settings.Store) Flags {
	f := Flags{CouchEnabled: true, RankingsEnabled: true, DownloadsEnabled: true}
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

// DownloadsOn is an httpx.Guard check for asking for downloads: preparing them costs the
// server disk and CPU. Downloads already prepared can still be fetched and removed.
func DownloadsOn(st *settings.Store) func(context.Context) error {
	return func(ctx context.Context) error {
		if !Load(ctx, st).DownloadsEnabled {
			return httpx.Fail(http.StatusNotFound, "feature_disabled", "downloads are disabled")
		}
		return nil
	}
}

// CouchTvOn is an httpx.Guard check that hides the Couch TV viewer routes while the
// channel is off. Its admin operations stay reachable so it can be set up first.
func CouchTvOn(st *settings.Store) func(context.Context) error {
	return func(ctx context.Context) error {
		if !Load(ctx, st).CouchTvEnabled {
			return httpx.Fail(http.StatusNotFound, "feature_disabled", "couch tv is disabled")
		}
		return nil
	}
}
