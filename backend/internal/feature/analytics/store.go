// Package analytics owns the watch time rollup behind the admin overview
// charts. Catalog calls RecordWatch from its existing progress beacon.
package analytics

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

// tzName is the zone the hour buckets are cut in, so "night" means night rather
// than whatever UTC happens to be. Go reports the placeholder "Local" when TZ is
// unset, which Postgres rejects.
func tzName() string {
	if n := time.Local.String(); n != "" && n != "Local" {
		return n
	}
	return "UTC"
}

// recordHour keeps the hour-of-day rollup behind the profile clock and the
// night-owl achievements. It rides along inside RecordWatch so no caller
// changes and the beacon keeps a single round trip per stat.
func (s *Store) recordHour(ctx context.Context, userID int64, seconds int) error {
	if seconds <= 0 {
		return nil
	}
	_, err := s.db.Exec(ctx,
		`INSERT INTO watch_time_hourly (day, hour, user_id, kind, seconds)
		 VALUES ((now() AT TIME ZONE $1)::date,
			extract(hour FROM now() AT TIME ZONE $1)::smallint, $2, 'video', $3)
		 ON CONFLICT (day, hour, user_id, kind)
		 DO UPDATE SET seconds = watch_time_hourly.seconds + EXCLUDED.seconds`,
		tzName(), userID, seconds)
	return err
}

// RecordWatch adds actually-played seconds for a movie (titleID) or an
// episode (episodeID, resolved to its title in SQL).
func (s *Store) RecordWatch(ctx context.Context, userID int64, titleID, episodeID *string, seconds int) error {
	if seconds <= 0 {
		return nil
	}
	_, err := s.db.Exec(ctx,
		`INSERT INTO watch_time_daily (day, user_id, title_id, kind, seconds)
		 VALUES (current_date, $1,
			COALESCE($2::uuid, (SELECT se.title_id FROM episodes e
				JOIN seasons se ON se.id = e.season_id WHERE e.id = $3::uuid)),
			'video', $4)
		 ON CONFLICT (day, user_id, kind, title_id)
		 DO UPDATE SET seconds = watch_time_daily.seconds + EXCLUDED.seconds`,
		userID, titleID, episodeID, seconds)
	if err != nil {
		return err
	}
	return s.recordHour(ctx, userID, seconds)
}

// RecordCouchWatch adds aggregate follower-seconds spent watching a title in a
// couch session. This is the separate "On Couch watch-time" stat: it is scoped
// per title (no user dimension), never counts toward normal watch-time, and is
// called best-effort by the couch hub.
func (s *Store) RecordCouchWatch(ctx context.Context, titleID string, seconds int) error {
	if seconds <= 0 || titleID == "" {
		return nil
	}
	_, err := s.db.Exec(ctx,
		`INSERT INTO couch_watch_time_daily (day, title_id, seconds)
		 VALUES (current_date, $1, $2)
		 ON CONFLICT (day, title_id)
		 DO UPDATE SET seconds = couch_watch_time_daily.seconds + EXCLUDED.seconds`,
		titleID, seconds)
	return err
}

// PruneHourly drops hour buckets past the profile clock's window. The daily
// rollup is kept forever (it is one row per user per day and the admin charts
// read it), but the hourly one is 48x denser and only ever read for the last year.
func (s *Store) PruneHourly(ctx context.Context, keepDays int) (int64, error) {
	tag, err := s.db.Exec(ctx,
		`DELETE FROM watch_time_hourly WHERE day < current_date - $1::int`, keepDays)
	return tag.RowsAffected(), err
}

// The overview types carry the feature name because API schema names share one
// namespace, where names like Totals or TopTitle are already taken.

type AnalyticsDay struct {
	Day          string `json:"day" format:"date"`
	VideoSeconds int64  `json:"videoSeconds"`
	CouchSeconds int64  `json:"couchSeconds" doc:"Seconds followers watched on a couch."`
	ActiveUsers  int    `json:"activeUsers"`
}

type AnalyticsTotals struct {
	VideoSeconds int64 `json:"videoSeconds"`
	CouchSeconds int64 `json:"couchSeconds" doc:"Seconds followers watched on a couch."`
	ActiveUsers  int   `json:"activeUsers"`
}

type AnalyticsTopTitle struct {
	TitleID string `json:"titleId" format:"uuid"`
	Slug    string `json:"slug"`
	Name    string `json:"name"`
	Kind    string `json:"kind" enum:"movie,series"`
	Seconds int64  `json:"seconds"`
}

type AnalyticsTopUser struct {
	UserID      int64   `json:"userId"`
	DisplayName string  `json:"displayName"`
	AvatarID    *string `json:"avatarId"`
	Seconds     int64   `json:"seconds"`
}

// AnalyticsOverview covers the last Days days up to today; Daily has one entry
// per day, oldest first.
type AnalyticsOverview struct {
	Days           int                 `json:"days"`
	Daily          []AnalyticsDay      `json:"daily"`
	Totals         AnalyticsTotals     `json:"totals"`
	TopTitles      []AnalyticsTopTitle `json:"topTitles"`
	TopCouchTitles []AnalyticsTopTitle `json:"topCouchTitles"`
	TopUsers       []AnalyticsTopUser  `json:"topUsers"`
}

func (s *Store) Overview(ctx context.Context, days int) (*AnalyticsOverview, error) {
	out := &AnalyticsOverview{Days: days, Daily: []AnalyticsDay{}, TopTitles: []AnalyticsTopTitle{},
		TopCouchTitles: []AnalyticsTopTitle{}, TopUsers: []AnalyticsTopUser{}}

	rows, err := s.db.Query(ctx,
		`SELECT d::date,
			COALESCE(sum(w.seconds) FILTER (WHERE w.kind = 'video'), 0),
			count(DISTINCT w.user_id),
			COALESCE((SELECT sum(c.seconds) FROM couch_watch_time_daily c WHERE c.day = d::date), 0)
		 FROM generate_series(current_date - ($1::int - 1), current_date, interval '1 day') d
		 LEFT JOIN watch_time_daily w ON w.day = d::date
		 GROUP BY d ORDER BY d`, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var d AnalyticsDay
		var day time.Time
		if err := rows.Scan(&day, &d.VideoSeconds, &d.ActiveUsers, &d.CouchSeconds); err != nil {
			return nil, err
		}
		d.Day = day.Format("2006-01-02")
		out.Daily = append(out.Daily, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	err = s.db.QueryRow(ctx,
		`SELECT COALESCE(sum(seconds) FILTER (WHERE kind = 'video'), 0),
			count(DISTINCT user_id)
		 FROM watch_time_daily WHERE day >= current_date - ($1::int - 1)`, days).
		Scan(&out.Totals.VideoSeconds, &out.Totals.ActiveUsers)
	if err != nil {
		return nil, err
	}

	if err := s.db.QueryRow(ctx,
		`SELECT COALESCE(sum(seconds), 0) FROM couch_watch_time_daily
		 WHERE day >= current_date - ($1::int - 1)`, days).
		Scan(&out.Totals.CouchSeconds); err != nil {
		return nil, err
	}

	titles, err := s.db.Query(ctx,
		`SELECT t.id, t.slug, t.name, t.kind, sum(w.seconds) AS secs
		 FROM watch_time_daily w
		 JOIN titles t ON t.id = w.title_id
		 WHERE w.kind = 'video' AND w.day >= current_date - ($1::int - 1)
		 GROUP BY t.id ORDER BY secs DESC LIMIT 10`, days)
	if err != nil {
		return nil, err
	}
	defer titles.Close()
	for titles.Next() {
		var t AnalyticsTopTitle
		if err := titles.Scan(&t.TitleID, &t.Slug, &t.Name, &t.Kind, &t.Seconds); err != nil {
			return nil, err
		}
		out.TopTitles = append(out.TopTitles, t)
	}
	if err := titles.Err(); err != nil {
		return nil, err
	}

	couchTitles, err := s.db.Query(ctx,
		`SELECT t.id, t.slug, t.name, t.kind, sum(c.seconds) AS secs
		 FROM couch_watch_time_daily c
		 JOIN titles t ON t.id = c.title_id
		 WHERE c.day >= current_date - ($1::int - 1)
		 GROUP BY t.id ORDER BY secs DESC LIMIT 10`, days)
	if err != nil {
		return nil, err
	}
	defer couchTitles.Close()
	for couchTitles.Next() {
		var t AnalyticsTopTitle
		if err := couchTitles.Scan(&t.TitleID, &t.Slug, &t.Name, &t.Kind, &t.Seconds); err != nil {
			return nil, err
		}
		out.TopCouchTitles = append(out.TopCouchTitles, t)
	}
	if err := couchTitles.Err(); err != nil {
		return nil, err
	}

	users, err := s.db.Query(ctx,
		`SELECT u.id, u.display_name, av.id, sum(w.seconds) AS secs
		 FROM watch_time_daily w
		 JOIN users u ON u.id = w.user_id
		 LEFT JOIN artwork av ON av.owner_kind = 'user' AND av.owner_id = u.id::text AND av.kind = 'avatar'
		 WHERE w.day >= current_date - ($1::int - 1)
		 GROUP BY u.id, av.id ORDER BY secs DESC LIMIT 10`, days)
	if err != nil {
		return nil, err
	}
	defer users.Close()
	for users.Next() {
		var u AnalyticsTopUser
		if err := users.Scan(&u.UserID, &u.DisplayName, &u.AvatarID, &u.Seconds); err != nil {
			return nil, err
		}
		out.TopUsers = append(out.TopUsers, u)
	}
	return out, users.Err()
}
