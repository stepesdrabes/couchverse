package store

import (
	"context"
	"fmt"
	"sort"
)

// UpsertProgress records playback position; >95% counts as completed.
func (s *Store) UpsertProgress(ctx context.Context, userID int64, titleID, episodeID *string, position, duration int) error {
	completed := duration > 0 && float64(position) >= float64(duration)*0.95

	if titleID != nil {
		_, err := s.pool.Exec(ctx,
			`INSERT INTO watch_progress (user_id, title_id, position_seconds, duration_seconds, completed)
			 VALUES ($1, $2, $3, $4, $5)
			 ON CONFLICT (user_id, title_id) WHERE title_id IS NOT NULL DO UPDATE
				SET position_seconds = EXCLUDED.position_seconds,
					duration_seconds = EXCLUDED.duration_seconds,
					completed = EXCLUDED.completed,
					updated_at = now()`,
			userID, *titleID, position, duration, completed)
		return err
	}
	_, err := s.pool.Exec(ctx,
		`INSERT INTO watch_progress (user_id, episode_id, position_seconds, duration_seconds, completed)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (user_id, episode_id) WHERE episode_id IS NOT NULL DO UPDATE
			SET position_seconds = EXCLUDED.position_seconds,
				duration_seconds = EXCLUDED.duration_seconds,
				completed = EXCLUDED.completed,
				updated_at = now()`,
		userID, *episodeID, position, duration, completed)
	return err
}

func (s *Store) ProgressFor(ctx context.Context, userID int64, titleID, episodeID *string) (position, duration int, err error) {
	err = s.pool.QueryRow(ctx,
		`SELECT position_seconds, duration_seconds FROM watch_progress
		 WHERE user_id = $1 AND NOT completed
			AND (($2::uuid IS NOT NULL AND title_id = $2) OR ($3::uuid IS NOT NULL AND episode_id = $3))`,
		userID, titleID, episodeID).Scan(&position, &duration)
	if err != nil {
		return 0, 0, nil // no row = start from zero
	}
	return position, duration, nil
}

// ContinueWatching lists in-progress items, newest first. For series it keeps
// only the most recently watched episode per show.
func (s *Store) ContinueWatching(ctx context.Context, userID int64, limit int) ([]ContinueItem, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT ON (t.id)
			t.id, t.slug, t.kind, t.name, t.year,
			(SELECT a.id FROM artwork a WHERE a.owner_kind = 'title' AND a.owner_id = t.id::text AND a.kind = 'poster'),
			(SELECT a.id FROM artwork a WHERE a.owner_kind = 'title' AND a.owner_id = t.id::text AND a.kind = 'backdrop'),
			e.id, se.season_number, e.episode_number, e.name,
			wp.position_seconds, wp.duration_seconds, wp.updated_at
		FROM watch_progress wp
		LEFT JOIN episodes e ON e.id = wp.episode_id
		LEFT JOIN seasons se ON se.id = e.season_id
		JOIN titles t ON t.id = COALESCE(wp.title_id, se.title_id)
		WHERE wp.user_id = $1 AND NOT wp.completed AND wp.position_seconds > 5
			AND t.status = 'published'
		ORDER BY t.id, wp.updated_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []ContinueItem{}
	for rows.Next() {
		var it ContinueItem
		var epID *string
		var seasonNum, epNum *int
		var epName *string
		if err := rows.Scan(&it.TitleID, &it.Slug, &it.Kind, &it.Name, &it.Year, &it.PosterID, &it.BackdropID,
			&epID, &seasonNum, &epNum, &epName, &it.Position, &it.Duration, &it.UpdatedAt); err != nil {
			return nil, err
		}
		if epID != nil {
			it.EpisodeID = epID
			it.PlaybackKind = "episode"
			it.PlaybackID = *epID
			label := ""
			if seasonNum != nil && epNum != nil {
				label = formatEpisodeLabel(*seasonNum, *epNum, deref(epName))
			}
			it.EpisodeLabel = label
		} else {
			it.PlaybackKind = "movie"
			it.PlaybackID = it.TitleID
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// newest activity first; DISTINCT ON forced title-id ordering
	sortContinueItems(items)
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func sortContinueItems(items []ContinueItem) {
	sort.Slice(items, func(i, j int) bool {
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
}

func formatEpisodeLabel(season, episode int, name string) string {
	label := fmt.Sprintf("S%d E%d", season, episode)
	if name != "" {
		label += " · " + name
	}
	return label
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Watchlist ("My List")

func (s *Store) WatchlistAdd(ctx context.Context, userID int64, titleID string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO watchlist (user_id, title_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		userID, titleID)
	return err
}

func (s *Store) WatchlistRemove(ctx context.Context, userID int64, titleID string) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM watchlist WHERE user_id = $1 AND title_id = $2`, userID, titleID)
	return err
}

func (s *Store) WatchlistHas(ctx context.Context, userID int64, titleID string) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM watchlist WHERE user_id = $1 AND title_id = $2)`,
		userID, titleID).Scan(&ok)
	return ok, err
}

func (s *Store) Watchlist(ctx context.Context, userID int64) ([]CardItem, error) {
	return s.scanCards(ctx, cardSelect+`
		JOIN watchlist w ON w.title_id = t.id
		WHERE w.user_id = $1 AND t.status = 'published'
		ORDER BY w.added_at DESC`, userID)
}

// EpisodeProgressForTitle maps episode id → (position, duration, completed)
// for the title detail page.
type EpisodeProgress struct {
	Position  int  `json:"positionSeconds"`
	Duration  int  `json:"durationSeconds"`
	Completed bool `json:"completed"`
}

func (s *Store) EpisodeProgressForTitle(ctx context.Context, userID int64, titleID string) (map[string]EpisodeProgress, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT wp.episode_id, wp.position_seconds, wp.duration_seconds, wp.completed
		 FROM watch_progress wp
		 JOIN episodes e ON e.id = wp.episode_id
		 JOIN seasons se ON se.id = e.season_id
		 WHERE wp.user_id = $1 AND se.title_id = $2`, userID, titleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]EpisodeProgress{}
	for rows.Next() {
		var id string
		var p EpisodeProgress
		if err := rows.Scan(&id, &p.Position, &p.Duration, &p.Completed); err != nil {
			return nil, err
		}
		out[id] = p
	}
	return out, rows.Err()
}
