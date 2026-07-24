package ranks

import (
	"testing"
	"time"

	"couchverse/internal/flags"
)

func allOn() flags.Flags {
	return flags.Flags{MusicEnabled: true, CouchEnabled: true, RankingsEnabled: true}
}

func byCode(list []Unlock) map[string]Unlock {
	out := make(map[string]Unlock, len(list))
	for _, u := range list {
		out[u.Code] = u
	}
	return out
}

func TestAchievementCatalogueIsWellFormed(t *testing.T) {
	categories := map[Category]bool{
		CatWatching: true, CatStreaks: true, CatExplorer: true,
		CatMusic: true, CatCouch: true, CatMeta: true,
	}
	seen := map[string]bool{}
	for _, a := range Achievements {
		if seen[a.Code] {
			t.Fatalf("duplicate achievement code %s", a.Code)
		}
		seen[a.Code] = true
		if a.Target <= 0 {
			t.Fatalf("%s has target %d, want a positive threshold", a.Code, a.Target)
		}
		if a.Value == nil {
			t.Fatalf("%s has no Value function", a.Code)
		}
		if !categories[a.Category] {
			t.Fatalf("%s has unknown category %q", a.Code, a.Category)
		}
		if defCfg.achievementXP(a.Tier) == 0 {
			t.Fatalf("%s has unknown tier %q", a.Code, a.Tier)
		}
		switch a.Requires {
		case "", needsMusic, needsCouch:
		default:
			t.Fatalf("%s requires unknown flag %q", a.Code, a.Requires)
		}
	}
}

// TestMetaTargetsAreReachable fails the moment someone adds a flag-gated
// achievement and bumps a meta target past what a music- or couch-disabled
// server can ever reach.
func TestMetaTargetsAreReachable(t *testing.T) {
	var alwaysVisible int64
	for _, a := range Achievements {
		if a.Requires == "" && a.Category != CatMeta {
			alwaysVisible++
		}
	}
	for _, a := range Achievements {
		if a.Category != CatMeta {
			continue
		}
		// only the rules that count other achievements are constrained
		if a.Value(Snapshot{AchievementsUnlocked: alwaysVisible}) != alwaysVisible {
			continue
		}
		if a.Target > alwaysVisible {
			t.Fatalf("%s needs %d achievements but only %d are reachable without feature flags",
				a.Code, a.Target, alwaysVisible)
		}
	}
}

func TestEvaluateUnlocksAtTarget(t *testing.T) {
	t.Run("one below the target stays locked", func(t *testing.T) {
		got := byCode(Evaluate(Snapshot{LongestStreak: 6}, nil, allOn(), defCfg))["streak_7"]
		if got.Unlocked {
			t.Fatal("streak_7 unlocked at 6 days")
		}
		if got.Value != 6 || got.Percent != 85 {
			t.Fatalf("got value=%d percent=%d, want 6/85", got.Value, got.Percent)
		}
	})

	t.Run("exactly the target unlocks", func(t *testing.T) {
		if got := byCode(Evaluate(Snapshot{LongestStreak: 7}, nil, allOn(), defCfg))["streak_7"]; !got.Unlocked {
			t.Fatal("streak_7 did not unlock at 7 days")
		}
	})

	t.Run("value and percent are capped", func(t *testing.T) {
		got := byCode(Evaluate(Snapshot{LongestStreak: 900}, nil, allOn(), defCfg))["streak_7"]
		if got.Value != 7 || got.Percent != 100 {
			t.Fatalf("got value=%d percent=%d, want 7/100", got.Value, got.Percent)
		}
	})
}

func TestEvaluateHidesFlagGatedAchievements(t *testing.T) {
	got := byCode(Evaluate(Snapshot{}, nil, flags.Flags{MusicEnabled: false, CouchEnabled: true}, defCfg))
	if _, ok := got["listen_50h"]; ok {
		t.Fatal("a music achievement is visible with music disabled")
	}
	if _, ok := got["couch_host_1"]; !ok {
		t.Fatal("a couch achievement is hidden with couch enabled")
	}

	got = byCode(Evaluate(Snapshot{}, nil, flags.Flags{MusicEnabled: true, CouchEnabled: false}, defCfg))
	if _, ok := got["couch_host_1"]; ok {
		t.Fatal("a couch achievement is visible with couch disabled")
	}
}

// TestEvaluateCountsMetaInTheSamePass is the two-pass contract: the badge that
// rewards ten badges must unlock in the very call that earns the tenth.
func TestEvaluateCountsMetaInTheSamePass(t *testing.T) {
	// A snapshot generous enough to clear well over ten non-meta rules.
	snap := Snapshot{
		XPInputs:       XPInputs{VideoSeconds: 40 * 3600, MusicSeconds: 60 * 3600, MoviesCompleted: 30, EpisodesCompleted: 200},
		DistinctTitles: 60, DistinctGenres: 12, DistinctDecades: 5, WatchlistSize: 20,
		LongestStreak: 40, SeriesCompleted: 2, BestDayMinutes: 400,
		NightNights: 12, EarlyMornings: 12, TracksPlayed: 200,
	}
	got := byCode(Evaluate(snap, nil, allOn(), defCfg))
	if !got["achievements_10"].Unlocked {
		t.Fatalf("achievements_10 locked at %d/%d", got["achievements_10"].Value, got["achievements_10"].Target)
	}
}

func TestEvaluatePreservesUnlockedAt(t *testing.T) {
	when := time.Date(2026, 3, 1, 20, 0, 0, 0, time.UTC)
	got := byCode(Evaluate(Snapshot{DistinctTitles: 1}, map[string]time.Time{"first_play": when}, allOn(), defCfg))

	if at := got["first_play"].UnlockedAt; at == nil || !at.Equal(when) {
		t.Fatalf("got %v, want %v", at, when)
	}
	// satisfied but not yet persisted: unlocked with no timestamp, which is
	// exactly what the check endpoint looks for
	if u := got["watchlist_10"]; u.Unlocked || u.UnlockedAt != nil {
		t.Fatalf("got unlocked=%v at=%v, want false/nil", u.Unlocked, u.UnlockedAt)
	}
}

func TestAchievementXPFor(t *testing.T) {
	count, xp := AchievementXPFor([]string{"first_play", "watch_500h", "gone_in_a_past_build"}, defCfg)
	if count != 2 {
		t.Fatalf("got count %d, want 2 (unknown codes are ignored)", count)
	}
	if want := defCfg.achievementXP(Bronze) + defCfg.achievementXP(Platinum); xp != want {
		t.Fatalf("got %d xp, want %d", xp, want)
	}
}
