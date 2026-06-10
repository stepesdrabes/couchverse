package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"couchverse/internal/httpx"
)

type Season struct {
	ID           string    `json:"id"`
	TitleID      string    `json:"titleId"`
	SeasonNumber int       `json:"seasonNumber"`
	Name         string    `json:"name"`
	Overview     string    `json:"overview"`
	Episodes     []Episode `json:"episodes"`
}

type Episode struct {
	ID             string     `json:"id"`
	SeasonID       string     `json:"seasonId"`
	EpisodeNumber  int        `json:"episodeNumber"`
	Name           string     `json:"name"`
	Overview       string     `json:"overview"`
	AirDate        *time.Time `json:"airDate"`
	RuntimeMinutes *int       `json:"runtimeMinutes"`
}

func scanSeason(row pgx.Row) (*Season, error) {
	var se Season
	err := row.Scan(&se.ID, &se.TitleID, &se.SeasonNumber, &se.Name, &se.Overview)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	se.Episodes = []Episode{}
	return &se, nil
}

func scanEpisode(row pgx.Row) (*Episode, error) {
	var e Episode
	err := row.Scan(&e.ID, &e.SeasonID, &e.EpisodeNumber, &e.Name, &e.Overview, &e.AirDate, &e.RuntimeMinutes)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// SeasonsWithEpisodes returns a title's seasons with episodes, ordered.
func (s *Store) SeasonsWithEpisodes(ctx context.Context, titleID string) ([]Season, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, title_id, season_number, name, overview
		 FROM seasons WHERE title_id = $1 ORDER BY season_number`, titleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	seasons := []Season{}
	byID := map[string]int{}
	for rows.Next() {
		se, err := scanSeason(rows)
		if err != nil {
			return nil, err
		}
		byID[se.ID] = len(seasons)
		seasons = append(seasons, *se)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(seasons) == 0 {
		return seasons, nil
	}

	erows, err := s.pool.Query(ctx,
		`SELECT e.id, e.season_id, e.episode_number, e.name, e.overview, e.air_date, e.runtime_minutes
		 FROM episodes e JOIN seasons se ON se.id = e.season_id
		 WHERE se.title_id = $1 ORDER BY e.episode_number`, titleID)
	if err != nil {
		return nil, err
	}
	defer erows.Close()
	for erows.Next() {
		e, err := scanEpisode(erows)
		if err != nil {
			return nil, err
		}
		if i, ok := byID[e.SeasonID]; ok {
			seasons[i].Episodes = append(seasons[i].Episodes, *e)
		}
	}
	return seasons, erows.Err()
}

func (s *Store) CreateSeason(ctx context.Context, titleID string, seasonNumber int, name string) (*Season, error) {
	return scanSeason(s.pool.QueryRow(ctx,
		`INSERT INTO seasons (title_id, season_number, name) VALUES ($1, $2, $3)
		 RETURNING id, title_id, season_number, name, overview`,
		titleID, seasonNumber, name))
}

func (s *Store) DeleteSeason(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM seasons WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return httpx.ErrNotFound
	}
	return nil
}

type EpisodeInput struct {
	EpisodeNumber  int    `json:"episodeNumber"`
	Name           string `json:"name"`
	Overview       string `json:"overview"`
	RuntimeMinutes *int   `json:"runtimeMinutes"`
}

func (s *Store) CreateEpisode(ctx context.Context, seasonID string, in EpisodeInput) (*Episode, error) {
	return scanEpisode(s.pool.QueryRow(ctx,
		`INSERT INTO episodes (season_id, episode_number, name, overview, runtime_minutes)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, season_id, episode_number, name, overview, air_date, runtime_minutes`,
		seasonID, in.EpisodeNumber, in.Name, in.Overview, in.RuntimeMinutes))
}

type EpisodeUpdate struct {
	EpisodeNumber  *int    `json:"episodeNumber"`
	Name           *string `json:"name"`
	Overview       *string `json:"overview"`
	RuntimeMinutes *int    `json:"runtimeMinutes"`
}

func (s *Store) UpdateEpisode(ctx context.Context, id string, up EpisodeUpdate) (*Episode, error) {
	return scanEpisode(s.pool.QueryRow(ctx,
		`UPDATE episodes SET
			episode_number = COALESCE($2, episode_number),
			name = COALESCE($3, name),
			overview = COALESCE($4, overview),
			runtime_minutes = COALESCE($5, runtime_minutes)
		 WHERE id = $1
		 RETURNING id, season_id, episode_number, name, overview, air_date, runtime_minutes`,
		id, up.EpisodeNumber, up.Name, up.Overview, up.RuntimeMinutes))
}

func (s *Store) DeleteEpisode(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM episodes WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return httpx.ErrNotFound
	}
	return nil
}
