package ranks

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Counter keys. Every counter is a monotonic bigint written by the couch hub and
// read by the snapshot builder, which is why one generic table beats a column
// per stat.
const (
	counterCouchHosted = "couch_hosted"
	counterCouchJoined = "couch_joined"
	counterCouchEmoji  = "couch_emoji"
	counterCouchParty  = "couch_party_max"
)

// publicFilter is the privacy predicate. The obvious
// (preferences->>'publicProfile')::boolean would raise a cast error if any
// client ever wrote a non-boolean there, taking the whole leaderboard down; the
// jsonb comparison cannot throw and fails open to public.
const publicFilter = `u.preferences -> 'publicProfile' IS DISTINCT FROM 'false'::jsonb`

type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

// addCounter bumps a summed counter. maxCounter keeps a high-water mark instead,
// which makes "biggest couch" idempotent under the hub's 20s flush.
func (s *Store) addCounter(ctx context.Context, userID int64, key string, delta int64) error {
	if delta <= 0 {
		return nil
	}
	_, err := s.db.Exec(ctx,
		`INSERT INTO user_counters (user_id, key, value) VALUES ($1, $2, $3)
		 ON CONFLICT (user_id, key)
		 DO UPDATE SET value = user_counters.value + EXCLUDED.value, updated_at = now()`,
		userID, key, delta)
	return err
}

func (s *Store) maxCounter(ctx context.Context, userID int64, key string, value int64) error {
	if value <= 0 {
		return nil
	}
	_, err := s.db.Exec(ctx,
		`INSERT INTO user_counters (user_id, key, value) VALUES ($1, $2, $3)
		 ON CONFLICT (user_id, key)
		 DO UPDATE SET value = GREATEST(user_counters.value, EXCLUDED.value), updated_at = now()`,
		userID, key, value)
	return err
}

// The four methods below satisfy couch.CouchStatsRecorder. They stay live even
// when the rankings flag is off, so turning it back on does not present an empty
// history.
func (s *Store) RecordCouchHosted(ctx context.Context, userID int64) error {
	return s.addCounter(ctx, userID, counterCouchHosted, 1)
}

func (s *Store) RecordCouchJoined(ctx context.Context, userID int64) error {
	return s.addCounter(ctx, userID, counterCouchJoined, 1)
}

func (s *Store) RecordCouchEmoji(ctx context.Context, userID int64, count int) error {
	return s.addCounter(ctx, userID, counterCouchEmoji, int64(count))
}

func (s *Store) RecordCouchPartySize(ctx context.Context, userID int64, size int) error {
	return s.maxCounter(ctx, userID, counterCouchParty, int64(size))
}

// UnlockedAt returns when each already-earned achievement was first unlocked.
func (s *Store) UnlockedAt(ctx context.Context, userID int64) (map[string]time.Time, error) {
	rows, err := s.db.Query(ctx,
		`SELECT code, unlocked_at FROM user_achievements WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]time.Time{}
	for rows.Next() {
		var code string
		var at time.Time
		if err := rows.Scan(&code, &at); err != nil {
			return nil, err
		}
		out[code] = at
	}
	return out, rows.Err()
}

// PersistUnlocks records the given codes and returns only the ones that were not
// already there. RETURNING after ON CONFLICT DO NOTHING yields exactly the rows
// actually inserted, so two tabs racing produce one celebration between them
// with no read-then-write window and no transaction.
func (s *Store) PersistUnlocks(ctx context.Context, userID int64, codes []string) (map[string]time.Time, error) {
	fresh := map[string]time.Time{}
	if len(codes) == 0 {
		return fresh, nil
	}
	rows, err := s.db.Query(ctx,
		`INSERT INTO user_achievements (user_id, code)
		 SELECT $1, unnest($2::text[])
		 ON CONFLICT DO NOTHING
		 RETURNING code, unlocked_at`, userID, codes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var code string
		var at time.Time
		if err := rows.Scan(&code, &at); err != nil {
			return nil, err
		}
		fresh[code] = at
	}
	return fresh, rows.Err()
}

// SnapshotFor gathers every measured fact about one user. Seven sequential
// queries against user-indexed columns, in the style of analytics.Overview: on a
// pool-local database that is well under a millisecond of latency each, and it
// keeps the SQL readable and debuggable one statement at a time.
func (s *Store) SnapshotFor(ctx context.Context, userID int64) (Snapshot, error) {
	var snap Snapshot

	// Watch and listen seconds, active days, both streaks and the best single
	// day. Streaks use gaps-and-islands: consecutive dates minus their row
	// number collapse to a shared anchor, so each unbroken run is one group.
	err := s.db.QueryRow(ctx,
		`WITH mine AS (
			SELECT day, kind, seconds FROM watch_time_daily WHERE user_id = $1
		), days AS (
			SELECT DISTINCT day FROM mine
		), grp AS (
			SELECT day, day - (row_number() OVER (ORDER BY day))::int AS anchor FROM days
		), runs AS (
			SELECT count(*) AS len, max(day) AS last_day FROM grp GROUP BY anchor
		)
		SELECT
			COALESCE((SELECT sum(seconds) FILTER (WHERE kind = 'video') FROM mine), 0)::bigint,
			COALESCE((SELECT sum(seconds) FILTER (WHERE kind = 'music') FROM mine), 0)::bigint,
			(SELECT count(*) FROM days),
			COALESCE((SELECT max(len) FROM runs), 0),
			COALESCE((SELECT max(len) FROM runs WHERE last_day >= current_date - 1), 0),
			COALESCE((SELECT max(s) FROM (
				SELECT sum(seconds) AS s FROM mine WHERE kind = 'video' GROUP BY day) d), 0)::bigint`,
		userID).Scan(&snap.VideoSeconds, &snap.MusicSeconds, &snap.ActiveDays,
		&snap.LongestStreak, &snap.CurrentStreak, &snap.BestDayMinutes)
	if err != nil {
		return snap, err
	}
	snap.BestDayMinutes /= 60

	// Completions and how many distinct titles have ever been started. Episodes
	// reach their title only through seasons.
	if err := s.db.QueryRow(ctx,
		`SELECT
			count(*) FILTER (WHERE wp.title_id IS NOT NULL AND wp.completed),
			count(*) FILTER (WHERE wp.episode_id IS NOT NULL AND wp.completed),
			count(DISTINCT COALESCE(wp.title_id, se.title_id))
		 FROM watch_progress wp
		 LEFT JOIN episodes e ON e.id = wp.episode_id
		 LEFT JOIN seasons se ON se.id = e.season_id
		 WHERE wp.user_id = $1`,
		userID).Scan(&snap.MoviesCompleted, &snap.EpisodesCompleted, &snap.DistinctTitles); err != nil {
		return snap, err
	}

	// Breadth: genres touched and decades of release covered.
	if err := s.db.QueryRow(ctx,
		`SELECT count(DISTINCT tg.genre_id), count(DISTINCT (t.year / 10))
		 FROM watch_progress wp
		 LEFT JOIN episodes e ON e.id = wp.episode_id
		 LEFT JOIN seasons se ON se.id = e.season_id
		 JOIN titles t ON t.id = COALESCE(wp.title_id, se.title_id)
		 LEFT JOIN title_genres tg ON tg.title_id = t.id
		 WHERE wp.user_id = $1`,
		userID).Scan(&snap.DistinctGenres, &snap.DistinctDecades); err != nil {
		return snap, err
	}

	// Series where every non-special episode is completed. Series with no
	// episodes at all are excluded, otherwise an empty draft would count.
	if err := s.db.QueryRow(ctx,
		`SELECT count(*) FROM titles t
		 WHERE t.kind = 'series' AND t.status = 'published'
		   AND EXISTS (SELECT 1 FROM seasons s JOIN episodes e ON e.season_id = s.id
		               WHERE s.title_id = t.id AND s.season_number > 0)
		   AND NOT EXISTS (
		       SELECT 1 FROM seasons s JOIN episodes e ON e.season_id = s.id
		       WHERE s.title_id = t.id AND s.season_number > 0
		         AND NOT EXISTS (SELECT 1 FROM watch_progress wp
		                         WHERE wp.user_id = $1 AND wp.episode_id = e.id AND wp.completed))`,
		userID).Scan(&snap.SeriesCompleted); err != nil {
		return snap, err
	}

	// Nights and early mornings, from the hour buckets.
	if err := s.db.QueryRow(ctx,
		`SELECT count(DISTINCT day) FILTER (WHERE hour < 5),
		        count(DISTINCT day) FILTER (WHERE hour BETWEEN 5 AND 7)
		 FROM watch_time_hourly WHERE user_id = $1`,
		userID).Scan(&snap.NightNights, &snap.EarlyMornings); err != nil {
		return snap, err
	}

	// Music reach, the biggest playlist and the watchlist.
	if err := s.db.QueryRow(ctx,
		`SELECT
			(SELECT count(*) FROM play_history WHERE user_id = $1 AND track_id IS NOT NULL),
			(SELECT count(DISTINCT al.artist_id) FROM play_history ph
				JOIN tracks tr ON tr.id = ph.track_id
				JOIN albums al ON al.id = tr.album_id
				WHERE ph.user_id = $1),
			COALESCE((SELECT max(c) FROM (
				SELECT count(*) AS c FROM playlist_tracks pt
				JOIN playlists p ON p.id = pt.playlist_id
				WHERE p.user_id = $1 GROUP BY p.id) x), 0),
			(SELECT count(*) FROM watchlist WHERE user_id = $1)`,
		userID).Scan(&snap.TracksPlayed, &snap.DistinctArtists, &snap.BiggestPlaylist, &snap.WatchlistSize); err != nil {
		return snap, err
	}

	// Account age and whether an avatar was ever uploaded.
	if err := s.db.QueryRow(ctx,
		`SELECT floor(extract(epoch FROM now() - u.created_at) / 86400)::bigint,
		        EXISTS (SELECT 1 FROM artwork a
		                WHERE a.owner_kind = 'user' AND a.owner_id = u.id::text AND a.kind = 'avatar')
		 FROM users u WHERE u.id = $1`,
		userID).Scan(&snap.AccountDays, &snap.HasAvatar); err != nil {
		return snap, err
	}

	counters, err := s.countersFor(ctx, userID)
	if err != nil {
		return snap, err
	}
	snap.CouchHosted = counters[counterCouchHosted]
	snap.CouchJoined = counters[counterCouchJoined]
	snap.EmojiSent = counters[counterCouchEmoji]
	snap.CouchPartyMax = counters[counterCouchParty]

	return snap, nil
}

func (s *Store) countersFor(ctx context.Context, userID int64) (map[string]int64, error) {
	rows, err := s.db.Query(ctx, `SELECT key, value FROM user_counters WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var k string
		var v int64
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}
