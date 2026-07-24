package ranks

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"couchverse/internal/db"
	"couchverse/internal/feature/catalog"
	"couchverse/internal/httpx"
)

// activityDays is the window of the profile heatmap: 53 whole weeks so the grid
// is a clean 53 x 7 with no ragged first column.
const activityDays = 371

// UserRef is the identity half of a profile, shared by the profile payload and
// the leaderboard rows. BannerAccent is the colour extracted from the banner when
// it was uploaded, so the hero can tint itself without a second request.
type UserRef struct {
	Username     string    `json:"username"`
	DisplayName  string    `json:"displayName"`
	AvatarID     *string   `json:"avatarId"`
	BannerID     *string   `json:"bannerId"`
	BannerAccent string    `json:"bannerAccent"`
	Bio          string    `json:"bio"`
	MemberSince  time.Time `json:"memberSince"`
}

// Target is a resolved profile subject plus whether they consent to being seen.
type Target struct {
	ID     int64
	Ref    UserRef
	Public bool
}

const targetSelect = `
	SELECT u.id, u.username, u.display_name, av.id, bn.id, COALESCE(bn.accent, ''), u.bio,
	       u.created_at, ` + publicFilter + `
	FROM users u
	LEFT JOIN artwork av ON av.owner_kind = 'user' AND av.owner_id = u.id::text AND av.kind = 'avatar'
	LEFT JOIN artwork bn ON bn.owner_kind = 'user' AND bn.owner_id = u.id::text AND bn.kind = 'banner'
	WHERE NOT u.disabled AND `

func scanTarget(row pgx.Row) (Target, error) {
	var t Target
	err := row.Scan(&t.ID, &t.Ref.Username, &t.Ref.DisplayName, &t.Ref.AvatarID, &t.Ref.BannerID,
		&t.Ref.BannerAccent, &t.Ref.Bio, &t.Ref.MemberSince, &t.Public)
	if errors.Is(err, pgx.ErrNoRows) {
		return t, db.ErrNotFound
	}
	return t, err
}

// TargetByUsername resolves a public profile subject. username is citext, so the
// lookup is already case-insensitive.
func (s *Store) TargetByUsername(ctx context.Context, username string) (Target, error) {
	return scanTarget(s.db.QueryRow(ctx, targetSelect+`u.username = $1`, username))
}

func (s *Store) TargetByID(ctx context.Context, id int64) (Target, error) {
	return scanTarget(s.db.QueryRow(ctx, targetSelect+`u.id = $1`, id))
}

type TopTitle struct {
	Slug     string  `json:"slug"`
	Name     string  `json:"name"`
	Kind     string  `json:"kind"`
	PosterID *string `json:"posterId"`
	Seconds  int64   `json:"seconds"`
}

// TopTitles are the titles this user has spent the most time on. Names are
// localized through catalog so the profile reads in the visitor's language.
func (s *Store) TopTitles(ctx context.Context, userID int64, limit int) ([]TopTitle, error) {
	rows, err := s.db.Query(ctx,
		`SELECT t.slug, t.name, t.kind, t.translations, po.id, sum(w.seconds)::bigint AS secs
		 FROM watch_time_daily w
		 JOIN titles t ON t.id = w.title_id
		 LEFT JOIN artwork po ON po.owner_kind = 'title' AND po.owner_id = t.id::text AND po.kind = 'poster'
		 WHERE w.user_id = $1 AND w.kind = 'video' AND t.status = 'published'
		 GROUP BY t.id, po.id ORDER BY secs DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TopTitle{}
	for rows.Next() {
		var t TopTitle
		var translations []byte
		if err := rows.Scan(&t.Slug, &t.Name, &t.Kind, &translations, &t.PosterID, &t.Seconds); err != nil {
			return nil, err
		}
		catalog.Localize(ctx, translations, &t.Name, nil)
		out = append(out, t)
	}
	return out, rows.Err()
}

// FavouriteGenre is the genre this user has watched most, already labelled for
// the request language (the English name stays the identity everywhere else).
func (s *Store) FavouriteGenre(ctx context.Context, userID int64) (string, error) {
	var name string
	err := s.db.QueryRow(ctx,
		`SELECT g.name FROM watch_time_daily w
		 JOIN title_genres tg ON tg.title_id = w.title_id
		 JOIN genres g ON g.id = tg.genre_id
		 WHERE w.user_id = $1 AND w.kind = 'video'
		 GROUP BY g.id ORDER BY sum(w.seconds) DESC LIMIT 1`, userID).Scan(&name)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return catalog.GenreLabel(name, httpx.LangFrom(ctx)), nil
}

// Activity is the calendar heatmap: a dense run of daily seconds ending today.
type Activity struct {
	From string  `json:"from"` // YYYY-MM-DD of days[0]
	Days []int64 `json:"days"`
}

func (s *Store) Activity(ctx context.Context, userID int64) (Activity, error) {
	out := Activity{Days: make([]int64, 0, activityDays)}
	rows, err := s.db.Query(ctx,
		`SELECT d::date, COALESCE(sum(w.seconds), 0)::bigint
		 FROM generate_series(current_date - ($2::int - 1), current_date, interval '1 day') d
		 LEFT JOIN watch_time_daily w ON w.day = d::date AND w.user_id = $1
		 GROUP BY d ORDER BY d`, userID, activityDays)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var day time.Time
		var secs int64
		if err := rows.Scan(&day, &secs); err != nil {
			return out, err
		}
		if out.From == "" {
			out.From = day.Format("2006-01-02")
		}
		out.Days = append(out.Days, secs)
	}
	return out, rows.Err()
}

// HourBucket is one slice of the "when you watch" clock.
type HourBucket struct {
	Hour         int   `json:"hour"`
	VideoSeconds int64 `json:"videoSeconds"`
	MusicSeconds int64 `json:"musicSeconds"`
}

// HourBuckets always returns 24 entries so the clock never has to densify.
func (s *Store) HourBuckets(ctx context.Context, userID int64) ([]HourBucket, error) {
	out := make([]HourBucket, 24)
	for i := range out {
		out[i].Hour = i
	}
	rows, err := s.db.Query(ctx,
		`SELECT hour,
			COALESCE(sum(seconds) FILTER (WHERE kind = 'video'), 0)::bigint,
			COALESCE(sum(seconds) FILTER (WHERE kind = 'music'), 0)::bigint
		 FROM watch_time_hourly WHERE user_id = $1 GROUP BY hour`, userID)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var hour int
		var video, music int64
		if err := rows.Scan(&hour, &video, &music); err != nil {
			return out, err
		}
		if hour >= 0 && hour < 24 {
			out[hour].VideoSeconds = video
			out[hour].MusicSeconds = music
		}
	}
	return out, rows.Err()
}
