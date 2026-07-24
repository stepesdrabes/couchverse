package ranks

// The shipped rates live in DefaultConfig: music is worth half of video because
// a background playlist should not outrank a movie night, and couch pays per
// session rather than per minute because on-couch watch-time has no user
// dimension in the schema. An admin can retune all of it.

// XPInputs are the countable facts the formula needs. The leaderboard loads
// exactly these for every user in four queries, while a profile view gets them
// as part of the fuller Snapshot.
type XPInputs struct {
	VideoSeconds      int64
	MusicSeconds      int64
	MoviesCompleted   int64
	EpisodesCompleted int64
	CouchHosted       int64
	CouchJoined       int64
	AchievementCount  int64
	AchievementXP     int64
}

// XPSource is one explainable line of the breakdown ("watching, 640 minutes,
// 2 XP each"). Key is a translation key, never a display string; Rate is 0 for
// achievements because theirs varies by tier.
type XPSource struct {
	Key   string `json:"key"`
	Units int64  `json:"units"`
	Rate  int64  `json:"rate"`
	XP    int64  `json:"xp"`
}

type XPResult struct {
	Total   int64      `json:"total"`
	Sources []XPSource `json:"sources"`
}

// ComputeXP turns the inputs into a total plus the breakdown behind it. Sources
// with no XP are dropped so the profile never renders an empty row, and the
// total is always the sum of what is returned.
func ComputeXP(in XPInputs, cfg Config) XPResult {
	out := XPResult{Sources: []XPSource{}}
	add := func(key string, units, rate int64) {
		xp := units * rate
		if xp <= 0 {
			return
		}
		out.Sources = append(out.Sources, XPSource{Key: key, Units: units, Rate: rate, XP: xp})
		out.Total += xp
	}

	add("video", in.VideoSeconds/60, cfg.Rates.VideoMinute)
	add("music", in.MusicSeconds/60, cfg.Rates.MusicMinute)
	add("movies", in.MoviesCompleted, cfg.Rates.Movie)
	add("episodes", in.EpisodesCompleted, cfg.Rates.Episode)
	add("couchHosted", in.CouchHosted, cfg.Rates.CouchHost)
	add("couchJoined", in.CouchJoined, cfg.Rates.CouchJoin)

	// achievements carry no single rate (bronze to platinum all differ), so the
	// units are the badges earned and the XP comes in pre-summed.
	if in.AchievementXP > 0 {
		out.Sources = append(out.Sources, XPSource{Key: "achievements", Units: in.AchievementCount, XP: in.AchievementXP})
		out.Total += in.AchievementXP
	}

	return out
}
