// Package ranks owns player progression: XP, rank tiers, achievements, public
// profiles and the global leaderboard. It reads every other feature's tables by
// SQL join rather than by import, so nothing but auth is imported here and
// nothing imports ranks in return.
package ranks

// RankTier is one rung of the rank ladder. Codes are persisted in payloads and
// translated by the frontend, so they are never renamed or renumbered.
type RankTier struct {
	Code   string `json:"code" enum:"rookie,remote,snack,binger,popcorn,marathoner,sage,cinephile,master,legend"`
	Level  int    `json:"level"`
	MinXP  int64  `json:"minXp"`
	Colour string `json:"colour"`
}

// NextRankTier is the tier above the current one; null at the top of the
// ladder, which is why it is a distinct (nullable) schema.
type NextRankTier struct {
	_ struct{} `nullable:"true"`
	RankTier
}

// defaultTiers is the shipped ladder. Codes, levels and colours are fixed (they
// are persisted in payloads and translated on the frontend); the thresholds are
// admin-tunable and come back through RankConfig.TierTable. Must stay sorted by
// MinXP ascending with Level equal to the 1-based index - TierFor and
// ProgressFor both rely on it and a test guards it.
var defaultTiers = []RankTier{
	{"rookie", 1, 0, "#7c8496"},
	{"remote", 2, 500, "#60a5fa"},
	{"snack", 3, 1500, "#22d3ee"},
	{"binger", 4, 3500, "#34d399"},
	{"popcorn", 5, 7000, "#a3e635"},
	{"marathoner", 6, 13000, "#facc15"},
	{"sage", 7, 23000, "#fb923c"},
	{"cinephile", 8, 40000, "#f87171"},
	{"master", 9, 70000, "#c084fc"},
	{"legend", 10, 120000, "#e879f9"},
}

// TierFor returns the highest tier the given XP has reached.
func TierFor(xp int64, tiers []RankTier) RankTier {
	t := tiers[0]
	for _, c := range tiers[1:] {
		if xp < c.MinXP {
			break
		}
		t = c
	}
	return t
}

// RankProgress is everything the rank ring needs: where you are, what is next and
// how far into the current tier you have climbed.
type RankProgress struct {
	XP       int64         `json:"xp"`
	Tier     RankTier      `json:"tier"`
	Next     *NextRankTier `json:"next"`
	IntoTier int64         `json:"intoTier" doc:"XP earned inside the current tier."`
	TierSpan int64         `json:"tierSpan" doc:"XP between this tier and the next; 0 at the top tier."`
	Percent  int           `json:"percent" minimum:"0" maximum:"100" doc:"Progress through the current tier; 100 at the top tier."`
}

// ProgressFor scores xp against the ladder. Negative XP cannot happen but is
// clamped rather than trusted, since it would otherwise produce a negative bar.
func ProgressFor(xp int64, tiers []RankTier) RankProgress {
	if xp < 0 {
		xp = 0
	}
	t := TierFor(xp, tiers)
	p := RankProgress{XP: xp, Tier: t, IntoTier: xp - t.MinXP, Percent: 100}
	if t.Level < len(tiers) {
		next := tiers[t.Level] // Level is 1-based, so this indexes the next tier
		p.Next = &NextRankTier{RankTier: next}
		p.TierSpan = next.MinXP - t.MinXP
		p.Percent = int(p.IntoTier * 100 / p.TierSpan)
	}
	return p
}
