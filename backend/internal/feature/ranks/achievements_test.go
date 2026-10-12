package ranks

import (
	"testing"
	"time"

	"couchverse/internal/flags"
)

func allOn() flags.Flags {
	return flags.Flags{CouchEnabled: true, RankingsEnabled: true, CouchTvEnabled: true}
}

func byCode(list []AchievementProgress) map[string]AchievementProgress {
	out := make(map[string]AchievementProgress, len(list))
	for _, u := range list {
		out[u.Code] = u
	}
	return out
}

func TestAchievementCatalogueIsWellFormed(t *testing.T) {
	categories := map[Category]bool{
		CatWatching: true, CatStreaks: true, CatExplorer: true,
		CatCouch: true, CatTv: true, CatMeta: true,
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
		case "", needsCouch, needsCouchTv:
		default:
			t.Fatalf("%s requires unknown flag %q", a.Code, a.Requires)
		}
	}
}

// TestMetaTargetsAreReachable fails the moment someone adds a flag-gated
// achievement and bumps a meta target past what a couch-disabled server can
// ever reach.
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
	got := byCode(Evaluate(Snapshot{}, nil, flags.Flags{CouchEnabled: true}, defCfg))
	if _, ok := got["couch_host_1"]; !ok {
		t.Fatal("a couch achievement is hidden with couch enabled")
	}

	got = byCode(Evaluate(Snapshot{}, nil, flags.Flags{CouchEnabled: false}, defCfg))
	if _, ok := got["couch_host_1"]; ok {
		t.Fatal("a couch achievement is visible with couch disabled")
	}

	// every Couch TV badge goes with its flag, and only with its own
	for _, f := range []flags.Flags{{CouchEnabled: true}, {CouchTvEnabled: true}} {
		got := byCode(Evaluate(Snapshot{}, nil, f, defCfg))
		for _, a := range Achievements {
			if a.Category != CatTv {
				continue
			}
			if _, ok := got[a.Code]; ok != f.CouchTvEnabled {
				t.Fatalf("%s visible=%v with couchTvEnabled=%v", a.Code, ok, f.CouchTvEnabled)
			}
		}
	}
}

// TestCouchTvAchievements scores each Couch TV rule one below its target and at
// it, from the snapshot fact it reads.
func TestCouchTvAchievements(t *testing.T) {
	tests := []struct {
		code   string
		target int64
		set    func(s *Snapshot, n int64)
	}{
		{"tv_prime_time_5", 5, func(s *Snapshot, n int64) { s.TvPrimeNights = n }},
		{"tv_night_owl", 1, func(s *Snapshot, n int64) { s.TvLateNights = n }},
		{"tv_morning_5", 5, func(s *Snapshot, n int64) { s.TvMornings = n }},
		// three others: the audience counts the member too
		{"tv_full_house", 3, func(s *Snapshot, n int64) { s.TvAudienceMax = n + 1 }},
		{"tv_reactions_100", 100, func(s *Snapshot, n int64) { s.TvReactions = n }},
		{"tv_strip_5", 5, func(s *Snapshot, n int64) { s.TvStripRun = n }},
		{"tv_marathon", 1, func(s *Snapshot, n int64) { s.TvMarathons = n }},
		{"tv_premiere", 1, func(s *Snapshot, n int64) { s.TvPremieres = n }},
	}
	tested := map[string]bool{}
	for _, tt := range tests {
		tested[tt.code] = true
		t.Run(tt.code, func(t *testing.T) {
			var below, at Snapshot
			tt.set(&below, tt.target-1)
			tt.set(&at, tt.target)
			if got := byCode(Evaluate(below, nil, allOn(), defCfg))[tt.code]; got.Unlocked || got.Value != tt.target-1 {
				t.Fatalf("one below: unlocked=%v value=%d", got.Unlocked, got.Value)
			}
			if got := byCode(Evaluate(at, nil, allOn(), defCfg))[tt.code]; !got.Unlocked || got.Target != tt.target {
				t.Fatalf("at the target: unlocked=%v target=%d", got.Unlocked, got.Target)
			}
		})
	}
	for _, a := range Achievements {
		if a.Category == CatTv && !tested[a.Code] {
			t.Errorf("%s has no case here", a.Code)
		}
	}

	// alone on the channel is no audience at all, not minus one
	if got := byCode(Evaluate(Snapshot{TvAudienceMax: 0}, nil, allOn(), defCfg))["tv_full_house"]; got.Value != 0 {
		t.Fatalf("an empty audience reads %d", got.Value)
	}
}

// TestEvaluateCountsMetaInTheSamePass is the two-pass contract: the badge that
// rewards ten badges must unlock in the very call that earns the tenth.
func TestEvaluateCountsMetaInTheSamePass(t *testing.T) {
	// A snapshot generous enough to clear well over ten non-meta rules.
	snap := Snapshot{
		XPInputs:       XPInputs{VideoSeconds: 40 * 3600, MoviesCompleted: 30, EpisodesCompleted: 200},
		DistinctTitles: 60, DistinctGenres: 12, DistinctDecades: 5, WatchlistSize: 20,
		LongestStreak: 40, SeriesCompleted: 2, BestDayMinutes: 400,
		NightNights: 12, EarlyMornings: 12,
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
