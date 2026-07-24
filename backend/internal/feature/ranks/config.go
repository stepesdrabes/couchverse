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

// Rates is the XP formula, exposed so an admin can retune pacing without a
// rebuild. Everything is derived from activity, so changing a rate re-levels
// everyone on the next read rather than needing a migration.
type Rates struct {
	VideoMinute int64 `json:"videoMinute"`
	MusicMinute int64 `json:"musicMinute"`
	Movie       int64 `json:"movie"`
	Episode     int64 `json:"episode"`
	CouchHost   int64 `json:"couchHost"`
	CouchJoin   int64 `json:"couchJoin"`
	Bronze      int64 `json:"bronze"`
	Silver      int64 `json:"silver"`
	Gold        int64 `json:"gold"`
	Platinum    int64 `json:"platinum"`
}

// Config is the whole tunable surface: the rates plus the ladder thresholds.
// Tier codes are fixed (they are persisted in payloads and translated on the
// frontend); only the XP each one starts at is editable.
type Config struct {
	Rates Rates   `json:"rates"`
	Tiers []int64 `json:"tiers"`
}

// DefaultConfig is the shipped balance, and the fallback whenever the stored
// value is absent or malformed.
func DefaultConfig() Config {
	tiers := make([]int64, len(defaultTiers))
	for i, t := range defaultTiers {
		tiers[i] = t.MinXP
	}
	return Config{
		Rates: Rates{
			VideoMinute: 2,
			MusicMinute: 1,
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
func LoadConfig(ctx context.Context, st *settings.Store) Config {
	cfg := DefaultConfig()
	if st == nil {
		return cfg
	}
	raw, err := st.Get(ctx, settingsKey)
	if err != nil || raw == nil {
		return cfg
	}
	var stored Config
	if json.Unmarshal(raw, &stored) != nil {
		return cfg
	}
	if stored.Rates != (Rates{}) {
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
func (c Config) Validate() error {
	rates := []struct {
		name  string
		value int64
	}{
		{"videoMinute", c.Rates.VideoMinute}, {"musicMinute", c.Rates.MusicMinute},
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
func (c Config) TierTable() []Tier {
	out := make([]Tier, len(defaultTiers))
	copy(out, defaultTiers)
	for i := range out {
		if i < len(c.Tiers) {
			out[i].MinXP = c.Tiers[i]
		}
	}
	return out
}

// achievementXP maps a badge tier to its configured reward.
func (c Config) achievementXP(tier AchTier) int64 {
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
