package ranks

import (
	"regexp"
	"testing"
)

func TestTierFor(t *testing.T) {
	tests := []struct {
		name string
		xp   int64
		want string
	}{
		{"zero", 0, "rookie"},
		{"negative", -50, "rookie"},
		{"one below the second tier", 499, "rookie"},
		{"exactly the second tier", 500, "remote"},
		{"mid tier", 9000, "popcorn"},
		{"one below the top", 119999, "master"},
		{"exactly the top", 120000, "legend"},
		{"far above the top", 10_000_000, "legend"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TierFor(tt.xp, defCfg.TierTable()).Code; got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProgressFor(t *testing.T) {
	t.Run("at a threshold the bar is empty", func(t *testing.T) {
		p := ProgressFor(500, defCfg.TierTable())
		if p.IntoTier != 0 || p.Percent != 0 {
			t.Fatalf("got into=%d percent=%d, want 0/0", p.IntoTier, p.Percent)
		}
		if p.Next == nil || p.Next.Code != "snack" {
			t.Fatalf("got next %v, want snack", p.Next)
		}
	})

	t.Run("mid tier", func(t *testing.T) {
		// halfway between remote (500) and snack (1500)
		p := ProgressFor(1000, defCfg.TierTable())
		if p.IntoTier != 500 || p.TierSpan != 1000 || p.Percent != 50 {
			t.Fatalf("got into=%d span=%d percent=%d, want 500/1000/50", p.IntoTier, p.TierSpan, p.Percent)
		}
	})

	t.Run("top tier is full and has no next", func(t *testing.T) {
		p := ProgressFor(200_000, defCfg.TierTable())
		if p.Next != nil {
			t.Fatalf("got next %v, want nil", p.Next)
		}
		if p.Percent != 100 || p.TierSpan != 0 {
			t.Fatalf("got percent=%d span=%d, want 100/0", p.Percent, p.TierSpan)
		}
	})

	t.Run("negative xp is clamped", func(t *testing.T) {
		if p := ProgressFor(-1, defCfg.TierTable()); p.XP != 0 || p.IntoTier != 0 {
			t.Fatalf("got xp=%d into=%d, want 0/0", p.XP, p.IntoTier)
		}
	})
}

// TestTiersAreWellFormed guards the invariants TierFor and ProgressFor rely on,
// so a future edit to the ladder cannot silently break the rank ring.
func TestTiersAreWellFormed(t *testing.T) {
	hex := regexp.MustCompile(`^#[0-9a-f]{6}$`)
	seen := map[string]bool{}
	if defaultTiers[0].MinXP != 0 {
		t.Fatalf("first tier starts at %d, want 0", defaultTiers[0].MinXP)
	}
	for i, tier := range defaultTiers {
		if tier.Level != i+1 {
			t.Fatalf("%s has level %d at index %d", tier.Code, tier.Level, i)
		}
		if i > 0 && tier.MinXP <= defaultTiers[i-1].MinXP {
			t.Fatalf("%s minXp %d is not above %s", tier.Code, tier.MinXP, defaultTiers[i-1].Code)
		}
		if seen[tier.Code] {
			t.Fatalf("duplicate tier code %s", tier.Code)
		}
		seen[tier.Code] = true
		if !hex.MatchString(tier.Colour) {
			t.Fatalf("%s has colour %q, want #rrggbb", tier.Code, tier.Colour)
		}
	}
}
