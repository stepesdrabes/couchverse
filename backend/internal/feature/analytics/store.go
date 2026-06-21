// Package analytics owns the watch/listen time rollup behind the admin
// overview charts. Catalog and music call RecordWatch/RecordListen from
// their existing beacon and scrobble handlers.
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
	return err
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

// RecordListen counts a scrobble as the track's duration of listening time.
// Skipped tracks over-count slightly - fine for an admin chart.
func (s *Store) RecordListen(ctx context.Context, userID int64, trackID string) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO watch_time_daily (day, user_id, title_id, kind, seconds)
		 SELECT current_date, $1, NULL, 'music', t.duration_seconds
		 FROM tracks t WHERE t.id = $2 AND t.duration_seconds > 0
		 ON CONFLICT (day, user_id, kind, title_id)
		 DO UPDATE SET seconds = watch_time_daily.seconds + EXCLUDED.seconds`,
		userID, trackID)
	return err
}

type Day struct {
	Day          string `json:"day"` // YYYY-MM-DD
	VideoSeconds int64  `json:"videoSeconds"`
	MusicSeconds int64  `json:"musicSeconds"`
	CouchSeconds int64  `json:"couchSeconds"`
	ActiveUsers  int    `json:"activeUsers"`
}

type Totals struct {
	VideoSeconds int64 `json:"videoSeconds"`
	MusicSeconds int64 `json:"musicSeconds"`
	CouchSeconds int64 `json:"couchSeconds"`
	ActiveUsers  int   `json:"activeUsers"`
}

type TopTitle struct {
	TitleID string `json:"titleId"`
	Slug    string `json:"slug"`
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Seconds int64  `json:"seconds"`
}

type TopUser struct {
	UserID      int64   `json:"userId"`
	DisplayName string  `json:"displayName"`
	AvatarID    *string `json:"avatarId"`
	Seconds     int64   `json:"seconds"`
}

type Overview struct {
	Days           int        `json:"days"`
	Daily          []Day      `json:"daily"`
	Totals         Totals     `json:"totals"`
	TopTitles      []TopTitle `json:"topTitles"`
	TopCouchTitles []TopTitle `json:"topCouchTitles"`
	TopUsers       []TopUser  `json:"topUsers"`
}

func (s *Store) Overview(ctx context.Context, days int) (*Overview, error) {
	out := &Overview{Days: days, Daily: []Day{}, TopTitles: []TopTitle{}, TopCouchTitles: []TopTitle{}, TopUsers: []TopUser{}}

	rows, err := s.db.Query(ctx,
		`SELECT d::date,
			COALESCE(sum(w.seconds) FILTER (WHERE w.kind = 'video'), 0),
			COALESCE(sum(w.seconds) FILTER (WHERE w.kind = 'music'), 0),
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
		var d Day
		var day time.Time
		if err := rows.Scan(&day, &d.VideoSeconds, &d.MusicSeconds, &d.ActiveUsers, &d.CouchSeconds); err != nil {
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
			COALESCE(sum(seconds) FILTER (WHERE kind = 'music'), 0),
			count(DISTINCT user_id)
		 FROM watch_time_daily WHERE day >= current_date - ($1::int - 1)`, days).
		Scan(&out.Totals.VideoSeconds, &out.Totals.MusicSeconds, &out.Totals.ActiveUsers)
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
		var t TopTitle
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
		var t TopTitle
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
		var u TopUser
		if err := users.Scan(&u.UserID, &u.DisplayName, &u.AvatarID, &u.Seconds); err != nil {
			return nil, err
		}
		out.TopUsers = append(out.TopUsers, u)
	}
	return out, users.Err()
}
