package ranks

import (
	"time"

	"couchverse/internal/flags"
)

type Category string

const (
	CatWatching Category = "watching"
	CatStreaks  Category = "streaks"
	CatExplorer Category = "explorer"
	CatMusic    Category = "music"
	CatCouch    Category = "couch"
	CatMeta     Category = "meta"
)

type AchTier string

const (
	Bronze   AchTier = "bronze"
	Silver   AchTier = "silver"
	Gold     AchTier = "gold"
	Platinum AchTier = "platinum"
)

// Feature gates. An achievement whose Requires is unmet is absent from the
// payload rather than shown locked, so a music-disabled server never displays
// an unreachable badge.
const (
	needsMusic = "music"
	needsCouch = "couch"
)

// Snapshot is every measured fact about one user. A single batch of SQL in the
// store fills it so the rules below stay pure and unit-testable.
type Snapshot struct {
	XPInputs

	ActiveDays     int64
	CurrentStreak  int64
	LongestStreak  int64
	BestDayMinutes int64
	NightNights    int64
	EarlyMornings  int64

	SeriesCompleted int64
	DistinctTitles  int64
	DistinctGenres  int64
	DistinctDecades int64
	WatchlistSize   int64

	TracksPlayed    int64
	DistinctArtists int64
	BiggestPlaylist int64

	EmojiSent     int64
	CouchPartyMax int64

	AccountDays int64
	HasAvatar   bool

	// AchievementsUnlocked is filled by Evaluate's first pass, never by SQL, so
	// meta rules can count the badges earned in the very same evaluation.
	AchievementsUnlocked int64
}

// Achievement is a pure rule: Value pulls one measured number out of the
// snapshot and Target is the threshold. Codes are persisted in user_achievements,
// so they are append-only and never renamed.
type Achievement struct {
	Code     string
	Category Category
	Tier     AchTier
	Target   int64
	Requires string
	Value    func(s Snapshot) int64
}

func boolValue(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// Achievements is the catalogue. Everything is derived from tables that already
// exist except the five couch rules (user_counters) and the two hour-of-day ones
// (watch_time_hourly).
var Achievements = []Achievement{
	// watching
	{"first_play", CatWatching, Bronze, 1, "", func(s Snapshot) int64 { return s.DistinctTitles }},
	{"watch_10h", CatWatching, Bronze, 600, "", func(s Snapshot) int64 { return s.VideoSeconds / 60 }},
	{"watch_50h", CatWatching, Silver, 3000, "", func(s Snapshot) int64 { return s.VideoSeconds / 60 }},
	{"watch_200h", CatWatching, Gold, 12000, "", func(s Snapshot) int64 { return s.VideoSeconds / 60 }},
	{"watch_500h", CatWatching, Platinum, 30000, "", func(s Snapshot) int64 { return s.VideoSeconds / 60 }},
	{"movies_25", CatWatching, Silver, 25, "", func(s Snapshot) int64 { return s.MoviesCompleted }},
	{"episodes_100", CatWatching, Silver, 100, "", func(s Snapshot) int64 { return s.EpisodesCompleted }},
	{"series_done_1", CatWatching, Bronze, 1, "", func(s Snapshot) int64 { return s.SeriesCompleted }},
	{"series_done_10", CatWatching, Gold, 10, "", func(s Snapshot) int64 { return s.SeriesCompleted }},
	{"marathon_6h", CatWatching, Silver, 360, "", func(s Snapshot) int64 { return s.BestDayMinutes }},

	// streaks
	{"streak_3", CatStreaks, Bronze, 3, "", func(s Snapshot) int64 { return s.LongestStreak }},
	{"streak_7", CatStreaks, Silver, 7, "", func(s Snapshot) int64 { return s.LongestStreak }},
	{"streak_30", CatStreaks, Gold, 30, "", func(s Snapshot) int64 { return s.LongestStreak }},
	{"night_owl_10", CatStreaks, Silver, 10, "", func(s Snapshot) int64 { return s.NightNights }},
	{"early_bird_10", CatStreaks, Bronze, 10, "", func(s Snapshot) int64 { return s.EarlyMornings }},
	{"active_days_50", CatStreaks, Silver, 50, "", func(s Snapshot) int64 { return s.ActiveDays }},
	{"active_days_200", CatStreaks, Gold, 200, "", func(s Snapshot) int64 { return s.ActiveDays }},

	// explorer
	{"genres_10", CatExplorer, Silver, 10, "", func(s Snapshot) int64 { return s.DistinctGenres }},
	{"titles_50", CatExplorer, Silver, 50, "", func(s Snapshot) int64 { return s.DistinctTitles }},
	{"watchlist_10", CatExplorer, Bronze, 10, "", func(s Snapshot) int64 { return s.WatchlistSize }},
	{"decades_4", CatExplorer, Silver, 4, "", func(s Snapshot) int64 { return s.DistinctDecades }},

	// music
	{"tracks_100", CatMusic, Bronze, 100, needsMusic, func(s Snapshot) int64 { return s.TracksPlayed }},
	{"listen_50h", CatMusic, Silver, 3000, needsMusic, func(s Snapshot) int64 { return s.MusicSeconds / 60 }},
	{"artists_25", CatMusic, Silver, 25, needsMusic, func(s Snapshot) int64 { return s.DistinctArtists }},
	{"playlist_50", CatMusic, Bronze, 50, needsMusic, func(s Snapshot) int64 { return s.BiggestPlaylist }},

	// couch
	{"couch_host_1", CatCouch, Bronze, 1, needsCouch, func(s Snapshot) int64 { return s.CouchHosted }},
	{"couch_host_25", CatCouch, Gold, 25, needsCouch, func(s Snapshot) int64 { return s.CouchHosted }},
	{"couch_join_10", CatCouch, Silver, 10, needsCouch, func(s Snapshot) int64 { return s.CouchJoined }},
	{"couch_party_6", CatCouch, Silver, 6, needsCouch, func(s Snapshot) int64 { return s.CouchPartyMax }},
	{"emoji_100", CatCouch, Bronze, 100, needsCouch, func(s Snapshot) int64 { return s.EmojiSent }},

	// meta
	{"avatar_set", CatMeta, Bronze, 1, "", func(s Snapshot) int64 { return boolValue(s.HasAvatar) }},
	{"veteran_365", CatMeta, Gold, 365, "", func(s Snapshot) int64 { return s.AccountDays }},
	{"achievements_10", CatMeta, Silver, 10, "", func(s Snapshot) int64 { return s.AchievementsUnlocked }},
	{"achievements_20", CatMeta, Platinum, 20, "", func(s Snapshot) int64 { return s.AchievementsUnlocked }},
}

// Unlock is one achievement scored against a snapshot, including locked progress
// so the UI can draw a progress ring on what is not earned yet.
type Unlock struct {
	Code       string     `json:"code"`
	Category   string     `json:"category"`
	Tier       string     `json:"tier"`
	XP         int64      `json:"xp"`
	Target     int64      `json:"target"`
	Value      int64      `json:"value"`
	Percent    int        `json:"percent"`
	Unlocked   bool       `json:"unlocked"`
	UnlockedAt *time.Time `json:"unlockedAt"`
}

// visible reports whether a rule's feature gate is satisfied.
func (a Achievement) visible(f flags.Flags) bool {
	switch a.Requires {
	case needsMusic:
		return f.MusicEnabled
	case needsCouch:
		return f.CouchEnabled
	default:
		return true
	}
}

func (a Achievement) score(s Snapshot, unlockedAt map[string]time.Time, cfg Config) Unlock {
	v := a.Value(s)
	if v < 0 {
		v = 0
	}
	u := Unlock{
		Code:     a.Code,
		Category: string(a.Category),
		Tier:     string(a.Tier),
		XP:       cfg.achievementXP(a.Tier),
		Target:   a.Target,
		Value:    min(v, a.Target),
		Unlocked: v >= a.Target,
	}
	u.Percent = int(u.Value * 100 / a.Target)
	if at, ok := unlockedAt[a.Code]; ok {
		when := at
		u.UnlockedAt = &when
	}
	return u
}

// Evaluate scores every visible rule against the snapshot. It runs in two passes
// so that the badge which unlocks your tenth badge unlocks in the same call:
// non-meta rules score first, their unlocked count feeds the snapshot, then the
// meta rules score. Pure, so the whole catalogue is testable without a database.
func Evaluate(s Snapshot, unlockedAt map[string]time.Time, f flags.Flags, cfg Config) []Unlock {
	out := make([]Unlock, 0, len(Achievements))
	var earned int64
	for _, a := range Achievements {
		if a.Category == CatMeta || !a.visible(f) {
			continue
		}
		u := a.score(s, unlockedAt, cfg)
		if u.Unlocked {
			earned++
		}
		out = append(out, u)
	}

	s.AchievementsUnlocked = earned
	for _, a := range Achievements {
		if a.Category != CatMeta || !a.visible(f) {
			continue
		}
		out = append(out, a.score(s, unlockedAt, cfg))
	}
	return out
}

// AchievementXPFor sums the reward of the codes a user has already unlocked.
// Flags are not consulted: disabling music must never demote anyone.
func AchievementXPFor(codes []string, cfg Config) (count, xp int64) {
	byCode := make(map[string]AchTier, len(Achievements))
	for _, a := range Achievements {
		byCode[a.Code] = a.Tier
	}
	for _, c := range codes {
		tier, ok := byCode[c]
		if !ok {
			continue // a code from an older build that no longer exists
		}
		count++
		xp += cfg.achievementXP(tier)
	}
	return count, xp
}
