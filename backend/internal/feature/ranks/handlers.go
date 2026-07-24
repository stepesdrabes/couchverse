package ranks

import (
	"context"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

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

// Totals are the headline numbers on a profile.
type Totals struct {
	VideoSeconds      int64 `json:"videoSeconds"`
	MusicSeconds      int64 `json:"musicSeconds"`
	MoviesCompleted   int64 `json:"moviesCompleted"`
	EpisodesCompleted int64 `json:"episodesCompleted"`
	SeriesCompleted   int64 `json:"seriesCompleted"`
	DistinctTitles    int64 `json:"distinctTitles"`
	DistinctGenres    int64 `json:"distinctGenres"`
	ActiveDays        int64 `json:"activeDays"`
	CurrentStreak     int64 `json:"currentStreak"`
	LongestStreak     int64 `json:"longestStreak"`
	BestDayMinutes    int64 `json:"bestDayMinutes"`
	TracksPlayed      int64 `json:"tracksPlayed"`
	DistinctArtists   int64 `json:"distinctArtists"`
	CouchHosted       int64 `json:"couchHosted"`
	CouchJoined       int64 `json:"couchJoined"`
	BiggestCouch      int64 `json:"biggestCouch"`
	EmojiSent         int64 `json:"emojiSent"`
}

func totalsOf(s Snapshot) Totals {
	return Totals{
		VideoSeconds:      s.VideoSeconds,
		MusicSeconds:      s.MusicSeconds,
		MoviesCompleted:   s.MoviesCompleted,
		EpisodesCompleted: s.EpisodesCompleted,
		SeriesCompleted:   s.SeriesCompleted,
		DistinctTitles:    s.DistinctTitles,
		DistinctGenres:    s.DistinctGenres,
		ActiveDays:        s.ActiveDays,
		CurrentStreak:     s.CurrentStreak,
		LongestStreak:     s.LongestStreak,
		BestDayMinutes:    s.BestDayMinutes,
		TracksPlayed:      s.TracksPlayed,
		DistinctArtists:   s.DistinctArtists,
		CouchHosted:       s.CouchHosted,
		CouchJoined:       s.CouchJoined,
		BiggestCouch:      s.CouchPartyMax,
		EmojiSent:         s.EmojiSent,
	}
}

// Profile is the public read model, shared by /me/stats and /users/{u}/profile.
type Profile struct {
	User            UserRef      `json:"user"`
	Rank            Progress     `json:"rank"`
	XP              XPResult     `json:"xp"`
	Totals          Totals       `json:"totals"`
	FavouriteGenre  string       `json:"favouriteGenre"`
	Achievements    []Unlock     `json:"achievements"`
	RecentUnlocks   []Unlock     `json:"recentUnlocks"`
	TopTitles       []TopTitle   `json:"topTitles"`
	Activity        Activity     `json:"activity"`
	Hours           []HourBucket `json:"hours"`
	AchievementsWon int          `json:"achievementsWon"`
	Public          bool         `json:"public"`
	IsSelf          bool         `json:"isSelf"`
}

// buildProfile is the single read path: it computes unlocks live from the
// snapshot and only reads persisted timestamps, so it can never steal a
// celebration from the check endpoint.
func (h *Handlers) buildProfile(ctx context.Context, t Target, isSelf bool) (*Profile, error) {
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

	return &Profile{
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
func recentUnlocks(all []Unlock) []Unlock {
	dated := make([]Unlock, 0, len(all))
	for _, u := range all {
		if u.Unlocked && u.UnlockedAt != nil {
			dated = append(dated, u)
		}
	}
	sort.Slice(dated, func(i, j int) bool { return dated[i].UnlockedAt.After(*dated[j].UnlockedAt) })
	return dated[:min(len(dated), recentUnlockLimit)]
}

// MyStats is the caller's own profile. It is the only progression call the app
// shell makes on load, feeding both the nav rank badge and the profile page.
func (h *Handlers) MyStats(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r.Context())
	target, err := h.store.TargetByID(r.Context(), user.ID)
	if err != nil {
		httpx.StoreErr(w, err)
		return
	}
	out, err := h.buildProfile(r.Context(), target, true)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

// PublicProfile serves someone else's profile. An opted-out member is a 404, not
// a 403, so "private" is indistinguishable from "no such member" - admins
// included, which is what makes the toggle an honest promise.
func (h *Handlers) PublicProfile(w http.ResponseWriter, r *http.Request) {
	target, err := h.store.TargetByUsername(r.Context(), chi.URLParam(r, "username"))
	if err != nil {
		httpx.StoreErr(w, err)
		return
	}
	self := auth.UserFrom(r.Context()).ID == target.ID
	if !target.Public && !self {
		httpx.NotFound(w)
		return
	}
	out, err := h.buildProfile(r.Context(), target, self)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

// CheckResult reports the badges this call persisted for the first time. Rank is
// null on a throttled call: the snapshot was skipped, so there is no fresh rank
// to report and the client must keep the one it has rather than paint a zero.
type CheckResult struct {
	Unlocked  []Unlock  `json:"unlocked"`
	Throttled bool      `json:"throttled"`
	Rank      *Progress `json:"rank"`
}

// Check is the single writer of user_achievements.
func (h *Handlers) Check(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r.Context())
	out := CheckResult{Unlocked: []Unlock{}}

	if !h.allowCheck(user.ID) {
		out.Throttled = true
		httpx.JSON(w, http.StatusOK, out)
		return
	}

	target, err := h.store.TargetByID(r.Context(), user.ID)
	if err != nil {
		httpx.StoreErr(w, err)
		return
	}
	profile, err := h.buildProfile(r.Context(), target, true)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	rank := profile.Rank
	out.Rank = &rank

	var earned []string
	for _, u := range profile.Achievements {
		if u.Unlocked && u.UnlockedAt == nil {
			earned = append(earned, u.Code)
		}
	}
	fresh, err := h.store.PersistUnlocks(r.Context(), user.ID, earned)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	for _, u := range profile.Achievements {
		if at, ok := fresh[u.Code]; ok {
			when := at
			u.UnlockedAt = &when
			out.Unlocked = append(out.Unlocked, u)
		}
	}
	httpx.JSON(w, http.StatusOK, out)
}

// LeaderRow carries every metric at once so the client can switch boards
// without a refetch. TierCode is always the all-time tier, so the badge means
// the same thing whichever metric is on screen.
type LeaderRow struct {
	Username     string  `json:"username"`
	DisplayName  string  `json:"displayName"`
	AvatarID     *string `json:"avatarId"`
	Level        int     `json:"level"`
	TierCode     string  `json:"tierCode"`
	XP           int64   `json:"xp"`
	WatchSeconds int64   `json:"watchSeconds"`
	MusicSeconds int64   `json:"musicSeconds"`
	Achievements int64   `json:"achievements"`
	IsSelf       bool    `json:"isSelf"`
}

// Leaderboard is unsorted on purpose: the client owns ordering because it owns
// the metric switch. Rows are the members who consent to being listed.
type Leaderboard struct {
	Period string      `json:"period"`
	Total  int         `json:"total"`
	Rows   []LeaderRow `json:"rows"`
	Me     *LeaderRow  `json:"me"`
	Hidden bool        `json:"hidden"`
}

// periodDays maps a period to a day window; 0 means all time.
func periodDays(period string) (string, int) {
	switch period {
	case "week":
		return "week", 7
	case "month":
		return "month", 30
	default:
		return "all", 0
	}
}

func (h *Handlers) Leaderboard(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r.Context())
	period, days := periodDays(r.URL.Query().Get("period"))
	cfg := LoadConfig(r.Context(), h.settings)
	tiers := cfg.TierTable()

	members, err := h.store.Members(r.Context())
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	inputs, err := h.store.XPRowsAll(r.Context(), cfg)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	watched, err := h.store.PeriodSeconds(r.Context(), "video", days)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	listened, err := h.store.PeriodSeconds(r.Context(), "music", days)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	unlocked, err := h.store.AchievementCounts(r.Context(), days)
	if err != nil {
		httpx.Internal(w, err)
		return
	}

	out := Leaderboard{Period: period, Rows: []LeaderRow{}}
	for _, m := range members {
		// XP is lifetime whatever the period: completions carry no date, so a
		// windowed XP number would be a fiction.
		xp := ComputeXP(inputs[m.ID], cfg).Total
		tier := TierFor(xp, tiers)
		row := LeaderRow{
			Username:     m.Ref.Username,
			DisplayName:  m.Ref.DisplayName,
			AvatarID:     m.Ref.AvatarID,
			Level:        tier.Level,
			TierCode:     tier.Code,
			XP:           xp,
			WatchSeconds: watched[m.ID],
			MusicSeconds: listened[m.ID],
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
	httpx.JSON(w, http.StatusOK, out)
}
