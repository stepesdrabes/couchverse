package ranks

import (
	"context"
)

// Member is a leaderboard candidate. Opted-out members are still loaded so the
// caller can be told their own score while staying off the public board.
type Member struct {
	ID     int64
	Ref    UserRef
	Public bool
}

// Members lists every enabled account. A self-hosted instance has a handful, so
// the whole set is loaded and ranked in Go rather than paged in SQL.
func (s *Store) Members(ctx context.Context) ([]Member, error) {
	rows, err := s.db.Query(ctx,
		`SELECT u.id, u.username, u.display_name, av.id, u.created_at, `+publicFilter+`
		 FROM users u
		 LEFT JOIN artwork av ON av.owner_kind = 'user' AND av.owner_id = u.id::text AND av.kind = 'avatar'
		 WHERE NOT u.disabled`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Member{}
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.ID, &m.Ref.Username, &m.Ref.DisplayName, &m.Ref.AvatarID, &m.Ref.MemberSince, &m.Public); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// XPRowsAll loads the XP inputs for every user at once. Each leaderboard row
// needs an all-time tier badge whatever the selected metric is, so this runs
// four grouped queries regardless of user count rather than a full Snapshot per
// member.
func (s *Store) XPRowsAll(ctx context.Context, cfg Config) (map[int64]XPInputs, error) {
	out := map[int64]XPInputs{}

	seconds, err := s.db.Query(ctx,
		`SELECT user_id,
			COALESCE(sum(seconds) FILTER (WHERE kind = 'video'), 0)::bigint,
			COALESCE(sum(seconds) FILTER (WHERE kind = 'music'), 0)::bigint
		 FROM watch_time_daily GROUP BY user_id`)
	if err != nil {
		return nil, err
	}
	defer seconds.Close()
	for seconds.Next() {
		var id, video, music int64
		if err := seconds.Scan(&id, &video, &music); err != nil {
			return nil, err
		}
		v := out[id]
		v.VideoSeconds, v.MusicSeconds = video, music
		out[id] = v
	}
	if err := seconds.Err(); err != nil {
		return nil, err
	}

	completions, err := s.db.Query(ctx,
		`SELECT user_id,
			count(*) FILTER (WHERE title_id IS NOT NULL AND completed),
			count(*) FILTER (WHERE episode_id IS NOT NULL AND completed)
		 FROM watch_progress GROUP BY user_id`)
	if err != nil {
		return nil, err
	}
	defer completions.Close()
	for completions.Next() {
		var id, movies, episodes int64
		if err := completions.Scan(&id, &movies, &episodes); err != nil {
			return nil, err
		}
		v := out[id]
		v.MoviesCompleted, v.EpisodesCompleted = movies, episodes
		out[id] = v
	}
	if err := completions.Err(); err != nil {
		return nil, err
	}

	counters, err := s.db.Query(ctx,
		`SELECT user_id, key, value FROM user_counters WHERE key = ANY($1)`,
		[]string{counterCouchHosted, counterCouchJoined})
	if err != nil {
		return nil, err
	}
	defer counters.Close()
	for counters.Next() {
		var id, value int64
		var key string
		if err := counters.Scan(&id, &key, &value); err != nil {
			return nil, err
		}
		v := out[id]
		switch key {
		case counterCouchHosted:
			v.CouchHosted = value
		case counterCouchJoined:
			v.CouchJoined = value
		}
		out[id] = v
	}
	if err := counters.Err(); err != nil {
		return nil, err
	}

	unlocked, err := s.db.Query(ctx, `SELECT user_id, code FROM user_achievements`)
	if err != nil {
		return nil, err
	}
	defer unlocked.Close()
	codes := map[int64][]string{}
	for unlocked.Next() {
		var id int64
		var code string
		if err := unlocked.Scan(&id, &code); err != nil {
			return nil, err
		}
		codes[id] = append(codes[id], code)
	}
	if err := unlocked.Err(); err != nil {
		return nil, err
	}
	for id, list := range codes {
		v := out[id]
		v.AchievementCount, v.AchievementXP = AchievementXPFor(list, cfg)
		out[id] = v
	}

	return out, nil
}

// UnlockCounts is how many members hold each achievement, for the admin rarity
// view. Opted-out members are counted: an admin is looking at the whole server.
func (s *Store) UnlockCounts(ctx context.Context) (map[string]int, error) {
	rows, err := s.db.Query(ctx,
		`SELECT code, count(*) FROM user_achievements GROUP BY code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var code string
		var n int
		if err := rows.Scan(&code, &n); err != nil {
			return nil, err
		}
		out[code] = n
	}
	return out, rows.Err()
}

// PeriodSeconds sums watch or listen seconds per user. days == 0 means all time.
func (s *Store) PeriodSeconds(ctx context.Context, kind string, days int) (map[int64]int64, error) {
	rows, err := s.db.Query(ctx,
		`SELECT user_id, COALESCE(sum(seconds), 0)::bigint FROM watch_time_daily
		 WHERE kind = $1 AND ($2::int = 0 OR day >= current_date - ($2::int - 1))
		 GROUP BY user_id`, kind, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanScores(rows)
}

// AchievementCounts counts unlocks per user. days == 0 means all time.
func (s *Store) AchievementCounts(ctx context.Context, days int) (map[int64]int64, error) {
	rows, err := s.db.Query(ctx,
		`SELECT user_id, count(*) FROM user_achievements
		 WHERE ($1::int = 0 OR unlocked_at >= now() - make_interval(days => $1))
		 GROUP BY user_id`, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanScores(rows)
}

type scoreRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}

func scanScores(rows scoreRows) (map[int64]int64, error) {
	out := map[int64]int64{}
	for rows.Next() {
		var id, score int64
		if err := rows.Scan(&id, &score); err != nil {
			return nil, err
		}
		out[id] = score
	}
	return out, rows.Err()
}
