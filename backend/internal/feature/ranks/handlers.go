package ranks

import (
	"context"
	"sort"
	"sync"
	"time"

	"couchverse/internal/feature/auth"
	"couchverse/internal/flags"
	"couchverse/internal/httpx"
	"couchverse/internal/settings"
)

// checkInterval throttles achievement evaluation per user. The client checks on
// app load, periodically during playback and when playback ends, and several
// tabs can fire at once; a throttled call reports nothing rather than re-running
// the snapshot.
const checkInterval = 30 * time.Second

const (
	topTitleLimit     = 5
	recentUnlockLimit = 5
)

type Handlers struct {
	store    *Store
	settings *settings.Store

	mu        sync.Mutex
	lastCheck map[int64]time.Time
}

func NewHandlers(st *Store, set *settings.Store) *Handlers {
	return &Handlers{store: st, settings: set, lastCheck: map[int64]time.Time{}}
}

// allowCheck reports whether this user's achievements may be re-evaluated now.
// The map is bounded by the number of accounts, so it needs no eviction.
func (h *Handlers) allowCheck(userID int64) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if t, ok := h.lastCheck[userID]; ok && time.Since(t) < checkInterval {
		return false
	}
	h.lastCheck[userID] = time.Now()
	return true
}

// ProfileTotals are the headline numbers on a profile.
type ProfileTotals struct {
	VideoSeconds      int64 `json:"videoSeconds"`
	MoviesCompleted   int64 `json:"moviesCompleted"`
	EpisodesCompleted int64 `json:"episodesCompleted"`
	SeriesCompleted   int64 `json:"seriesCompleted"`
	DistinctTitles    int64 `json:"distinctTitles"`
	DistinctGenres    int64 `json:"distinctGenres"`
	ActiveDays        int64 `json:"activeDays"`
	CurrentStreak     int64 `json:"currentStreak"`
	LongestStreak     int64 `json:"longestStreak"`
	BestDayMinutes    int64 `json:"bestDayMinutes"`
	CouchHosted       int64 `json:"couchHosted"`
	CouchJoined       int64 `json:"couchJoined"`
	BiggestCouch      int64 `json:"biggestCouch"`
	EmojiSent         int64 `json:"emojiSent"`
}

func totalsOf(s Snapshot) ProfileTotals {
	return ProfileTotals{
		VideoSeconds:      s.VideoSeconds,
		MoviesCompleted:   s.MoviesCompleted,
		EpisodesCompleted: s.EpisodesCompleted,
		SeriesCompleted:   s.SeriesCompleted,
		DistinctTitles:    s.DistinctTitles,
		DistinctGenres:    s.DistinctGenres,
		ActiveDays:        s.ActiveDays,
		CurrentStreak:     s.CurrentStreak,
		LongestStreak:     s.LongestStreak,
		BestDayMinutes:    s.BestDayMinutes,
		CouchHosted:       s.CouchHosted,
		CouchJoined:       s.CouchJoined,
		BiggestCouch:      s.CouchPartyMax,
		EmojiSent:         s.EmojiSent,
	}
}

// UserProfile is the public read model, shared by /me/stats and
// /users/{u}/profile.
type UserProfile struct {
	User            ProfileUser           `json:"user"`
	Rank            RankProgress          `json:"rank"`
	XP              XPResult              `json:"xp"`
	Totals          ProfileTotals         `json:"totals"`
	FavouriteGenre  string                `json:"favouriteGenre" doc:"Most watched genre's label in the display language; empty when none."`
	Achievements    []AchievementProgress `json:"achievements"`
	RecentUnlocks   []AchievementProgress `json:"recentUnlocks" doc:"Newest unlocked achievements first."`
	TopTitles       []ProfileTopTitle     `json:"topTitles"`
	Activity        ProfileActivity       `json:"activity"`
	Hours           []ProfileHourBucket   `json:"hours" doc:"Watch time by hour of day, always 24 entries."`
	AchievementsWon int                   `json:"achievementsWon"`
	Public          bool                  `json:"public" doc:"Whether the member appears on public profiles and leaderboards."`
	IsSelf          bool                  `json:"isSelf"`
}

// buildProfile is the single read path: it computes unlocks live from the
// snapshot and only reads persisted timestamps, so it can never steal a
// celebration from the check endpoint.
func (h *Handlers) buildProfile(ctx context.Context, t Target, isSelf bool) (*UserProfile, error) {
	cfg := LoadConfig(ctx, h.settings)
	snap, err := h.store.SnapshotFor(ctx, t.ID)
	if err != nil {
		return nil, err
	}
	unlockedAt, err := h.store.UnlockedAt(ctx, t.ID)
	if err != nil {
		return nil, err
	}
	codes := make([]string, 0, len(unlockedAt))
	for code := range unlockedAt {
		codes = append(codes, code)
	}
	snap.AchievementCount, snap.AchievementXP = AchievementXPFor(codes, cfg)

	f := flags.Load(ctx, h.settings)
	achievements := Evaluate(snap, unlockedAt, f, cfg)
	xp := ComputeXP(snap.XPInputs, cfg)

	top, err := h.store.TopTitles(ctx, t.ID, topTitleLimit)
	if err != nil {
		return nil, err
	}
	genre, err := h.store.FavouriteGenre(ctx, t.ID)
	if err != nil {
		return nil, err
	}
	activity, err := h.store.Activity(ctx, t.ID)
	if err != nil {
		return nil, err
	}
	hours, err := h.store.HourBuckets(ctx, t.ID)
	if err != nil {
		return nil, err
	}

	won := 0
	for _, u := range achievements {
		if u.Unlocked {
			won++
		}
	}

	return &UserProfile{
		User:            t.Ref,
		Rank:            ProgressFor(xp.Total, cfg.TierTable()),
		XP:              xp,
		Totals:          totalsOf(snap),
		FavouriteGenre:  genre,
		Achievements:    achievements,
		RecentUnlocks:   recentUnlocks(achievements),
		TopTitles:       top,
		Activity:        activity,
		Hours:           hours,
		AchievementsWon: won,
		Public:          t.Public,
		IsSelf:          isSelf,
	}, nil
}

// recentUnlocks are the newest badges, for the "just earned" strip.
func recentUnlocks(all []AchievementProgress) []AchievementProgress {
	dated := make([]AchievementProgress, 0, len(all))
	for _, u := range all {
		if u.Unlocked && u.UnlockedAt != nil {
			dated = append(dated, u)
		}
	}
	sort.Slice(dated, func(i, j int) bool { return dated[i].UnlockedAt.After(*dated[j].UnlockedAt) })
	return dated[:min(len(dated), recentUnlockLimit)]
}

type profileOutput struct{ Body *UserProfile }

// MyStats is the caller's own profile. It is the only progression call the app
// shell makes on load, feeding both the nav rank badge and the profile page.
func (h *Handlers) MyStats(ctx context.Context, _ *struct{}) (*profileOutput, error) {
	target, err := h.store.TargetByID(ctx, auth.UserFrom(ctx).ID)
	if err != nil {
		return nil, err
	}
	out, err := h.buildProfile(ctx, target, true)
	if err != nil {
		return nil, err
	}
	return &profileOutput{Body: out}, nil
}

type profileInput struct {
	Username string `path:"username"`
}

// PublicProfile serves someone else's profile. An opted-out member is a 404, not
// a 403, so "private" is indistinguishable from "no such member" - admins
// included, which is what makes the toggle an honest promise.
func (h *Handlers) PublicProfile(ctx context.Context, in *profileInput) (*profileOutput, error) {
	target, err := h.store.TargetByUsername(ctx, in.Username)
	if err != nil {
		return nil, err
	}
	self := auth.UserFrom(ctx).ID == target.ID
	if !target.Public && !self {
		return nil, httpx.NotFoundError()
	}
	out, err := h.buildProfile(ctx, target, self)
	if err != nil {
		return nil, err
	}
	return &profileOutput{Body: out}, nil
}

// CheckedRank is the caller's rank after a check. It is null on a throttled
// call: the snapshot was skipped, so there is no fresh rank to report and the
// client must keep the one it has rather than paint a zero.
type CheckedRank struct {
	_ struct{} `nullable:"true"`
	RankProgress
}

// AchievementCheck reports the badges this call persisted for the first time.
type AchievementCheck struct {
	Unlocked  []AchievementProgress `json:"unlocked"`
	Throttled bool                  `json:"throttled"`
	Rank      *CheckedRank          `json:"rank"`
}

type checkOutput struct{ Body AchievementCheck }

// Check is the single writer of user_achievements.
func (h *Handlers) Check(ctx context.Context, _ *struct{}) (*checkOutput, error) {
	user := auth.UserFrom(ctx)
	out := AchievementCheck{Unlocked: []AchievementProgress{}}

	if !h.allowCheck(user.ID) {
		out.Throttled = true
		return &checkOutput{Body: out}, nil
	}

	target, err := h.store.TargetByID(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	profile, err := h.buildProfile(ctx, target, true)
	if err != nil {
		return nil, err
	}
	out.Rank = &CheckedRank{RankProgress: profile.Rank}

	var earned []string
	for _, u := range profile.Achievements {
		if u.Unlocked && u.UnlockedAt == nil {
			earned = append(earned, u.Code)
		}
	}
	fresh, err := h.store.PersistUnlocks(ctx, user.ID, earned)
	if err != nil {
		return nil, err
	}
	for _, u := range profile.Achievements {
		if at, ok := fresh[u.Code]; ok {
			when := at
			u.UnlockedAt = &when
			out.Unlocked = append(out.Unlocked, u)
		}
	}
	return &checkOutput{Body: out}, nil
}

// LeaderboardRow carries every metric at once so the client can switch boards
// without a refetch. TierCode is always the all-time tier, so the badge means
// the same thing whichever metric is on screen.
type LeaderboardRow struct {
	Username     string  `json:"username"`
	DisplayName  string  `json:"displayName"`
	AvatarID     *string `json:"avatarId"`
	Level        int     `json:"level"`
	TierCode     string  `json:"tierCode" enum:"rookie,remote,snack,binger,popcorn,marathoner,sage,cinephile,master,legend"`
	XP           int64   `json:"xp"`
	WatchSeconds int64   `json:"watchSeconds" doc:"Watch time within the period."`
	Achievements int64   `json:"achievements" doc:"Achievements unlocked within the period."`
	IsSelf       bool    `json:"isSelf"`
}

// Leaderboard is unsorted on purpose: the client owns ordering because it owns
// the metric switch. Rows are the members who consent to being listed.
type Leaderboard struct {
	Period string           `json:"period" enum:"all,week,month"`
	Total  int              `json:"total"`
	Rows   []LeaderboardRow `json:"rows"`
	Me     *LeaderboardRow  `json:"me,omitempty" doc:"The caller's own row, listed or not."`
	Hidden bool             `json:"hidden" doc:"The caller opted out, so is missing from rows."`
}

type leaderboardInput struct {
	Period string `query:"period" enum:"all,week,month" default:"all"`
}

type leaderboardOutput struct{ Body Leaderboard }

// periodDays maps a period to a day window; 0 means all time.
func periodDays(period string) int {
	switch period {
	case "week":
		return 7
	case "month":
		return 30
	}
	return 0
}

func (h *Handlers) Leaderboard(ctx context.Context, in *leaderboardInput) (*leaderboardOutput, error) {
	user := auth.UserFrom(ctx)
	days := periodDays(in.Period)
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
	watched, err := h.store.WatchSeconds(ctx, days)
	if err != nil {
		return nil, err
	}
	unlocked, err := h.store.AchievementCounts(ctx, days)
	if err != nil {
		return nil, err
	}

	out := Leaderboard{Period: in.Period, Rows: []LeaderboardRow{}}
	for _, m := range members {
		// XP is lifetime whatever the period: completions carry no date, so a
		// windowed XP number would be a fiction.
		xp := ComputeXP(inputs[m.ID], cfg).Total
		tier := TierFor(xp, tiers)
		row := LeaderboardRow{
			Username:     m.Ref.Username,
			DisplayName:  m.Ref.DisplayName,
			AvatarID:     m.Ref.AvatarID,
			Level:        tier.Level,
			TierCode:     tier.Code,
			XP:           xp,
			WatchSeconds: watched[m.ID],
			Achievements: unlocked[m.ID],
			IsSelf:       m.ID == user.ID,
		}
		if !m.Public {
			// An opted-out member is listed nowhere. The caller still gets their
			// own row back so the page can say "you are hidden" with real numbers.
			if row.IsSelf {
				out.Hidden = true
				me := row
				out.Me = &me
			}
			continue
		}
		if row.IsSelf {
			me := row
			out.Me = &me
		}
		out.Rows = append(out.Rows, row)
	}
	out.Total = len(out.Rows)
	return &leaderboardOutput{Body: out}, nil
}
