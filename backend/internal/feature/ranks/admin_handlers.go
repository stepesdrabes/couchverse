package ranks

import (
	"context"
	"encoding/json"
	"sort"

	"couchverse/internal/flags"
	"couchverse/internal/httpx"
	"couchverse/internal/settings"
)

type AdminRanks struct {
	store    *Store
	settings *settings.Store
}

func NewAdminRanks(st *Store, set *settings.Store) *AdminRanks {
	return &AdminRanks{store: st, settings: set}
}

// MemberRank is one member's progression at a glance. This is the only place
// that reports an opted-out member's standing, since an admin needs to see the
// server as a whole.
type MemberRank struct {
	UserID       int64   `json:"userId"`
	Username     string  `json:"username"`
	DisplayName  string  `json:"displayName"`
	AvatarID     *string `json:"avatarId"`
	Level        int     `json:"level"`
	TierCode     string  `json:"tierCode" enum:"rookie,remote,snack,binger,popcorn,marathoner,sage,cinephile,master,legend"`
	XP           int64   `json:"xp"`
	Achievements int64   `json:"achievements"`
	WatchSeconds int64   `json:"watchSeconds"`
	CouchHosted  int64   `json:"couchHosted"`
	Public       bool    `json:"public"`
}

// AchievementStat is how many members hold a badge; low counts are the rare ones.
type AchievementStat struct {
	Code     string `json:"code"`
	Category string `json:"category" enum:"watching,streaks,explorer,couch,meta"`
	Tier     string `json:"tier" enum:"bronze,silver,gold,platinum"`
	Unlocked int    `json:"unlocked" doc:"How many members hold it."`
	Hidden   bool   `json:"hidden" doc:"Gated off by a feature flag right now."`
}

// TierBucket is one rung of the ladder with how many members stand on it.
type TierBucket struct {
	Code    string `json:"code" enum:"rookie,remote,snack,binger,popcorn,marathoner,sage,cinephile,master,legend"`
	Level   int    `json:"level"`
	MinXP   int64  `json:"minXp"`
	Colour  string `json:"colour"`
	Members int    `json:"members"`
}

// RanksOverview is the admin ranks dashboard.
type RanksOverview struct {
	Members      int               `json:"members"`
	TotalXP      int64             `json:"totalXp"`
	TotalUnlocks int               `json:"totalUnlocks"`
	AverageLevel float64           `json:"averageLevel"`
	Tiers        []TierBucket      `json:"tiers"`
	Achievements []AchievementStat `json:"achievements"`
	Rows         []MemberRank      `json:"rows"`
	Config       RankConfig        `json:"config"`
}

type overviewOutput struct{ Body RanksOverview }

// Overview is the admin ranks dashboard: who is where on the ladder, which
// badges are actually rare, and the config behind it.
func (h *AdminRanks) Overview(ctx context.Context, _ *struct{}) (*overviewOutput, error) {
	cfg := LoadConfig(ctx, h.settings)
	tiers := cfg.TierTable()

	members, err := h.store.Members(ctx)
	if err != nil {
		return nil, err
	}
	inputs, err := h.store.XPRowsAll(ctx, cfg)
	if err != nil {
		return nil, err
	}
	unlocks, err := h.store.UnlockCounts(ctx)
	if err != nil {
		return nil, err
	}
	perUser, err := h.store.AchievementCounts(ctx, 0)
	if err != nil {
		return nil, err
	}
	watched, err := h.store.WatchSeconds(ctx, 0)
	if err != nil {
		return nil, err
	}

	out := RanksOverview{
		Members:      len(members),
		Config:       cfg,
		Tiers:        []TierBucket{},
		Rows:         []MemberRank{},
		Achievements: []AchievementStat{},
	}

	byLevel := map[int]int{}
	for _, m := range members {
		in := inputs[m.ID]
		xp := ComputeXP(in, cfg).Total
		tier := TierFor(xp, tiers)
		byLevel[tier.Level]++
		out.TotalXP += xp
		out.Rows = append(out.Rows, MemberRank{
			UserID:       m.ID,
			Username:     m.Ref.Username,
			DisplayName:  m.Ref.DisplayName,
			AvatarID:     m.Ref.AvatarID,
			Level:        tier.Level,
			TierCode:     tier.Code,
			XP:           xp,
			Achievements: perUser[m.ID],
			WatchSeconds: watched[m.ID],
			CouchHosted:  in.CouchHosted,
			Public:       m.Public,
		})
	}
	sort.SliceStable(out.Rows, func(i, j int) bool { return out.Rows[i].XP > out.Rows[j].XP })

	for _, tier := range tiers {
		out.Tiers = append(out.Tiers, TierBucket{
			Code:    tier.Code,
			Level:   tier.Level,
			MinXP:   tier.MinXP,
			Colour:  tier.Colour,
			Members: byLevel[tier.Level],
		})
		out.AverageLevel += float64(tier.Level * byLevel[tier.Level])
	}
	if len(members) > 0 {
		out.AverageLevel /= float64(len(members))
	}

	f := flags.Load(ctx, h.settings)
	for _, a := range Achievements {
		count := unlocks[a.Code]
		out.TotalUnlocks += count
		out.Achievements = append(out.Achievements, AchievementStat{
			Code:     a.Code,
			Category: string(a.Category),
			Tier:     string(a.Tier),
			Unlocked: count,
			Hidden:   !a.visible(f),
		})
	}
	// rarest first: that is the interesting end of the list
	sort.SliceStable(out.Achievements, func(i, j int) bool {
		return out.Achievements[i].Unlocked < out.Achievements[j].Unlocked
	})

	return &overviewOutput{Body: out}, nil
}

type putConfigInput struct{ Body RankConfig }

type configOutput struct{ Body RankConfig }

// PutConfig stores a validated progression config. XP is always derived, so a
// saved change re-levels everyone on their next read - there is nothing to
// migrate or recompute.
func (h *AdminRanks) PutConfig(ctx context.Context, in *putConfigInput) (*configOutput, error) {
	cfg := in.Body
	if err := cfg.Validate(); err != nil {
		return nil, httpx.BadRequestError(err.Error())
	}
	body, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	if err := h.settings.Set(ctx, settingsKey, body); err != nil {
		return nil, err
	}
	return &configOutput{Body: cfg}, nil
}
