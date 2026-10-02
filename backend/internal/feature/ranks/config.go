package ranks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"couchverse/internal/settings"
)

// settingsKey is where the admin-tunable progression config lives.
const settingsKey = "ranks"

// RankRates is the XP formula, exposed so an admin can retune pacing without a
// rebuild. Everything is derived from activity, so changing a rate re-levels
// everyone on the next read rather than needing a migration.
type RankRates struct {
	VideoMinute int64 `json:"videoMinute" minimum:"0" doc:"XP per minute of video watched."`
	Movie       int64 `json:"movie" minimum:"0" doc:"XP per completed movie."`
	Episode     int64 `json:"episode" minimum:"0" doc:"XP per completed episode."`
	CouchHost   int64 `json:"couchHost" minimum:"0" doc:"XP per couch session hosted."`
	CouchJoin   int64 `json:"couchJoin" minimum:"0" doc:"XP per couch session joined."`
	Bronze      int64 `json:"bronze" minimum:"0" doc:"XP per bronze achievement."`
	Silver      int64 `json:"silver" minimum:"0" doc:"XP per silver achievement."`
	Gold        int64 `json:"gold" minimum:"0" doc:"XP per gold achievement."`
	Platinum    int64 `json:"platinum" minimum:"0" doc:"XP per platinum achievement."`
}

// RankConfig is the whole tunable surface: the rates plus the ladder thresholds.
// Tier codes are fixed (they are persisted in payloads and translated on the
// frontend); only the XP each one starts at is editable.
type RankConfig struct {
	Rates RankRates `json:"rates"`
	Tiers []int64   `json:"tiers" minItems:"10" maxItems:"10" doc:"XP each tier starts at, rookie first; starts at 0 and strictly ascends."`
}

// DefaultConfig is the shipped balance, and the fallback whenever the stored
// value is absent or malformed.
func DefaultConfig() RankConfig {
	tiers := make([]int64, len(defaultTiers))
	for i, t := range defaultTiers {
		tiers[i] = t.MinXP
	}
	return RankConfig{
		Rates: RankRates{
			VideoMinute: 2,
			Movie:       100,
			Episode:     20,
			CouchHost:   50,
			CouchJoin:   25,
			Bronze:      50,
			Silver:      150,
			Gold:        400,
			Platinum:    1000,
		},
		Tiers: tiers,
	}
}

// LoadConfig reads the stored config, falling back to the defaults field by
// field so a partially written value cannot zero out the whole formula.
func LoadConfig(ctx context.Context, st *settings.Store) RankConfig {
	cfg := DefaultConfig()
	if st == nil {
		return cfg
	}
	raw, err := st.Get(ctx, settingsKey)
	if err != nil || raw == nil {
		return cfg
	}
	var stored RankConfig
	if json.Unmarshal(raw, &stored) != nil {
		return cfg
	}
	if stored.Rates != (RankRates{}) {
		cfg.Rates = stored.Rates
	}
	if len(stored.Tiers) == len(cfg.Tiers) {
		cfg.Tiers = stored.Tiers
	}
	return cfg
}

// Validate rejects a config that would make the ladder nonsensical. Negative
// rates would let activity subtract XP, and a non-ascending ladder would break
// the "highest tier reached" scan.
func (c RankConfig) Validate() error {
	rates := []struct {
		name  string
		value int64
	}{
		{"videoMinute", c.Rates.VideoMinute},
		{"movie", c.Rates.Movie}, {"episode", c.Rates.Episode},
		{"couchHost", c.Rates.CouchHost}, {"couchJoin", c.Rates.CouchJoin},
		{"bronze", c.Rates.Bronze}, {"silver", c.Rates.Silver},
		{"gold", c.Rates.Gold}, {"platinum", c.Rates.Platinum},
	}
	for _, r := range rates {
		if r.value < 0 {
			return fmt.Errorf("%s must not be negative", r.name)
		}
	}
	if len(c.Tiers) != len(defaultTiers) {
		return fmt.Errorf("expected %d tier thresholds, got %d", len(defaultTiers), len(c.Tiers))
	}
	if c.Tiers[0] != 0 {
		return errors.New("the first tier must start at 0 xp")
	}
	for i := 1; i < len(c.Tiers); i++ {
		if c.Tiers[i] <= c.Tiers[i-1] {
			return fmt.Errorf("tier %d must require more xp than tier %d", i+1, i)
		}
	}
	return nil
}

// TierTable applies the configured thresholds to the fixed tier codes/colours.
func (c RankConfig) TierTable() []RankTier {
	out := make([]RankTier, len(defaultTiers))
	copy(out, defaultTiers)
	for i := range out {
		if i < len(c.Tiers) {
			out[i].MinXP = c.Tiers[i]
		}
	}
	return out
}

// achievementXP maps a badge tier to its configured reward.
func (c RankConfig) achievementXP(tier AchTier) int64 {
	switch tier {
	case Bronze:
		return c.Rates.Bronze
	case Silver:
		return c.Rates.Silver
	case Gold:
		return c.Rates.Gold
	case Platinum:
		return c.Rates.Platinum
	}
	return 0
}
