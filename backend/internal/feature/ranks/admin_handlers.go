package ranks

import (
	"encoding/json"
	"net/http"
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

// AdminMember is one member's progression at a glance. This is the only place
// that reports an opted-out member's standing, since an admin needs to see the
// server as a whole.
type AdminMember struct {
	UserID       int64   `json:"userId"`
	Username     string  `json:"username"`
	DisplayName  string  `json:"displayName"`
	AvatarID     *string `json:"avatarId"`
	Level        int     `json:"level"`
	TierCode     string  `json:"tierCode"`
	XP           int64   `json:"xp"`
	Achievements int64   `json:"achievements"`
	WatchSeconds int64   `json:"watchSeconds"`
	MusicSeconds int64   `json:"musicSeconds"`
	CouchHosted  int64   `json:"couchHosted"`
	Public       bool    `json:"public"`
}

// AchievementStat is how many members hold a badge; low counts are the rare ones.
type AchievementStat struct {
	Code     string `json:"code"`
	Category string `json:"category"`
	Tier     string `json:"tier"`
	Unlocked int    `json:"unlocked"`
	Hidden   bool   `json:"hidden"` // gated off by a feature flag right now
}

type TierBucket struct {
	Code    string `json:"code"`
	Level   int    `json:"level"`
	MinXP   int64  `json:"minXp"`
	Colour  string `json:"colour"`
	Members int    `json:"members"`
}

type AdminOverview struct {
	Members      int               `json:"members"`
	TotalXP      int64             `json:"totalXp"`
	TotalUnlocks int              `json:"totalUnlocks"`
	AverageLevel float64           `json:"averageLevel"`
	Tiers        []TierBucket      `json:"tiers"`
	Achievements []AchievementStat `json:"achievements"`
	Rows         []AdminMember     `json:"rows"`
	Config       Config            `json:"config"`
}

// Overview is the admin ranks dashboard: who is where on the ladder, which
// badges are actually rare, and the config behind it.
func (h *AdminRanks) Overview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	cfg := LoadConfig(ctx, h.settings)
	tiers := cfg.TierTable()

	members, err := h.store.Members(ctx)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	inputs, err := h.store.XPRowsAll(ctx, cfg)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	unlocks, err := h.store.UnlockCounts(ctx)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	perUser, err := h.store.AchievementCounts(ctx, 0)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	watched, err := h.store.PeriodSeconds(ctx, "video", 0)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	listened, err := h.store.PeriodSeconds(ctx, "music", 0)
	if err != nil {
		httpx.Internal(w, err)
		return
	}

	out := AdminOverview{
		Members:      len(members),
		Config:       cfg,
		Rows:         []AdminMember{},
		Achievements: []AchievementStat{},
	}

	byLevel := map[int]int{}
	for _, m := range members {
		in := inputs[m.ID]
		xp := ComputeXP(in, cfg).Total
		tier := TierFor(xp, tiers)
		byLevel[tier.Level]++
		out.TotalXP += xp
		out.Rows = append(out.Rows, AdminMember{
			UserID:       m.ID,
			Username:     m.Ref.Username,
			DisplayName:  m.Ref.DisplayName,
			AvatarID:     m.Ref.AvatarID,
			Level:        tier.Level,
			TierCode:     tier.Code,
			XP:           xp,
			Achievements: perUser[m.ID],
			WatchSeconds: watched[m.ID],
			MusicSeconds: listened[m.ID],
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

	httpx.JSON(w, http.StatusOK, out)
}

// PutConfig stores a validated progression config. XP is always derived, so a
// saved change re-levels everyone on their next read - there is nothing to
// migrate or recompute.
func (h *AdminRanks) PutConfig(w http.ResponseWriter, r *http.Request) {
	var cfg Config
	if err := httpx.Decode(r, &cfg); err != nil {
		httpx.BadRequest(w, "invalid request body")
		return
	}
	if err := cfg.Validate(); err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	body, err := json.Marshal(cfg)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	if err := h.settings.Set(r.Context(), settingsKey, body); err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, cfg)
}
