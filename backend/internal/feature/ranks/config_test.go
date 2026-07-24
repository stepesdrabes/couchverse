package ranks

import "testing"

// defCfg is the shipped balance, used by every test that does not care about
// admin retuning.
var defCfg = DefaultConfig()

func TestDefaultConfigIsValid(t *testing.T) {
	if err := DefaultConfig().Validate(); err != nil {
		t.Fatalf("the shipped defaults do not pass their own validation: %v", err)
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr bool
	}{
		{"defaults", func(*Config) {}, false},
		{"zero rate is allowed", func(c *Config) { c.Rates.MusicMinute = 0 }, false},
		{"negative rate", func(c *Config) { c.Rates.VideoMinute = -1 }, true},
		{"negative achievement reward", func(c *Config) { c.Rates.Gold = -5 }, true},
		{"first tier must be 0", func(c *Config) { c.Tiers[0] = 10 }, true},
		{"tiers must ascend", func(c *Config) { c.Tiers[3] = c.Tiers[2] }, true},
		{"wrong tier count", func(c *Config) { c.Tiers = c.Tiers[:5] }, true},
		{"rescaled ladder", func(c *Config) {
			for i := range c.Tiers {
				c.Tiers[i] *= 2
			}
		}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tt.mutate(&cfg)
			if err := cfg.Validate(); (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// A retuned config has to actually move people, otherwise the admin form is
// decorative.
func TestConfigChangesLevelling(t *testing.T) {
	in := XPInputs{VideoSeconds: 3600} // 60 minutes

	cfg := DefaultConfig()
	if got := ComputeXP(in, cfg).Total; got != 120 {
		t.Fatalf("got %d, want 120 at the default rate", got)
	}

	cfg.Rates.VideoMinute = 10
	if got := ComputeXP(in, cfg).Total; got != 600 {
		t.Fatalf("got %d, want 600 after retuning", got)
	}

	// halving every threshold should promote the same XP further up the ladder
	// (1000 xp sits in remote by default, but clears snack once halved)
	const xp = 1000
	base := TierFor(xp, DefaultConfig().TierTable())
	for i := range cfg.Tiers {
		cfg.Tiers[i] /= 2
	}
	if retuned := TierFor(xp, cfg.TierTable()); retuned.Level <= base.Level {
		t.Fatalf("got level %d, want above %d after halving the ladder", retuned.Level, base.Level)
	}
}

// The tier codes and colours are identity, not config: only the thresholds move.
func TestTierTableKeepsCodesAndColours(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Tiers[9] = 999_999
	table := cfg.TierTable()
	for i, tier := range table {
		if tier.Code != defaultTiers[i].Code || tier.Colour != defaultTiers[i].Colour {
			t.Fatalf("tier %d changed identity: %+v", i, tier)
		}
	}
	if table[9].MinXP != 999_999 {
		t.Fatalf("got %d, want the configured threshold", table[9].MinXP)
	}
}
