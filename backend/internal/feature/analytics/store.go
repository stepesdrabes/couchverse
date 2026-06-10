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
	ActiveUsers  int    `json:"activeUsers"`
}

type Totals struct {
	VideoSeconds int64 `json:"videoSeconds"`
	MusicSeconds int64 `json:"musicSeconds"`
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
	UserID      int64  `json:"userId"`
	DisplayName string `json:"displayName"`
	Seconds     int64  `json:"seconds"`
}

type Overview struct {
	Days      int        `json:"days"`
	Daily     []Day      `json:"daily"`
	Totals    Totals     `json:"totals"`
	TopTitles []TopTitle `json:"topTitles"`
	TopUsers  []TopUser  `json:"topUsers"`
}

func (s *Store) Overview(ctx context.Context, days int) (*Overview, error) {
	out := &Overview{Days: days, Daily: []Day{}, TopTitles: []TopTitle{}, TopUsers: []TopUser{}}

	rows, err := s.db.Query(ctx,
		`SELECT d::date,
			COALESCE(sum(w.seconds) FILTER (WHERE w.kind = 'video'), 0),
			COALESCE(sum(w.seconds) FILTER (WHERE w.kind = 'music'), 0),
			count(DISTINCT w.user_id)
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
		if err := rows.Scan(&day, &d.VideoSeconds, &d.MusicSeconds, &d.ActiveUsers); err != nil {
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

	users, err := s.db.Query(ctx,
		`SELECT u.id, u.display_name, sum(w.seconds) AS secs
		 FROM watch_time_daily w
		 JOIN users u ON u.id = w.user_id
		 WHERE w.day >= current_date - ($1::int - 1)
		 GROUP BY u.id ORDER BY secs DESC LIMIT 10`, days)
	if err != nil {
		return nil, err
	}
	defer users.Close()
	for users.Next() {
		var u TopUser
		if err := users.Scan(&u.UserID, &u.DisplayName, &u.Seconds); err != nil {
			return nil, err
		}
		out.TopUsers = append(out.TopUsers, u)
	}
	return out, users.Err()
}
