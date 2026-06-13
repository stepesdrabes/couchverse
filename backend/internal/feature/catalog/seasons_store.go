package catalog

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"couchverse/internal/db"
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
	ThumbID        *string    `json:"thumbId"`
	ThumbVer       int64      `json:"thumbVer,omitempty"`
}

func scanSeason(row pgx.Row) (*Season, error) {
	var se Season
	err := row.Scan(&se.ID, &se.TitleID, &se.SeasonNumber, &se.Name, &se.Overview)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, db.ErrNotFound
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
		return nil, db.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// SeasonsWithEpisodes returns a title's seasons with episodes, ordered.
func (s *Store) SeasonsWithEpisodes(ctx context.Context, titleID string) ([]Season, error) {
	rows, err := s.db.Query(ctx,
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

	erows, err := s.db.Query(ctx,
		`SELECT e.id, e.season_id, e.episode_number, e.name, e.overview, e.air_date, e.runtime_minutes,
			(SELECT a.id FROM artwork a
			 WHERE a.owner_kind = 'episode' AND a.owner_id = e.id::text AND a.kind = 'thumb'),
			COALESCE((SELECT extract(epoch FROM a.created_at)::bigint FROM artwork a
			 WHERE a.owner_kind = 'episode' AND a.owner_id = e.id::text AND a.kind = 'thumb'), 0)
		 FROM episodes e JOIN seasons se ON se.id = e.season_id
		 WHERE se.title_id = $1 ORDER BY e.episode_number`, titleID)
	if err != nil {
		return nil, err
	}
	defer erows.Close()
	for erows.Next() {
		var e Episode
		if err := erows.Scan(&e.ID, &e.SeasonID, &e.EpisodeNumber, &e.Name, &e.Overview,
			&e.AirDate, &e.RuntimeMinutes, &e.ThumbID, &e.ThumbVer); err != nil {
			return nil, err
		}
		if i, ok := byID[e.SeasonID]; ok {
			seasons[i].Episodes = append(seasons[i].Episodes, e)
		}
	}
	return seasons, erows.Err()
}

func (s *Store) CreateSeason(ctx context.Context, titleID string, seasonNumber int, name string) (*Season, error) {
	return scanSeason(s.db.QueryRow(ctx,
		`INSERT INTO seasons (title_id, season_number, name) VALUES ($1, $2, $3)
		 RETURNING id, title_id, season_number, name, overview`,
		titleID, seasonNumber, name))
}

func (s *Store) DeleteSeason(ctx context.Context, id string) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM seasons WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return db.ErrNotFound
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
	return scanEpisode(s.db.QueryRow(ctx,
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
	return scanEpisode(s.db.QueryRow(ctx,
		`UPDATE episodes SET
			episode_number = COALESCE($2, episode_number),
			name = COALESCE($3, name),
			overview = COALESCE($4, overview),
			runtime_minutes = COALESCE($5, runtime_minutes)
		 WHERE id = $1
		 RETURNING id, season_id, episode_number, name, overview, air_date, runtime_minutes`,
		id, up.EpisodeNumber, up.Name, up.Overview, up.RuntimeMinutes))
}

// ImportSeasonMeta upserts a season from TMDB, filling only placeholder or
// empty fields on existing rows.
func (s *Store) ImportSeasonMeta(ctx context.Context, titleID string, seasonNumber int, name, overview string) (string, error) {
	var id string
	err := s.db.QueryRow(ctx,
		`INSERT INTO seasons (title_id, season_number, name, overview)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (title_id, season_number) DO UPDATE SET
			name = CASE WHEN EXCLUDED.name = ''
					OR (seasons.name <> '' AND seasons.name <> 'Season ' || seasons.season_number)
				THEN seasons.name ELSE EXCLUDED.name END,
			overview = CASE WHEN EXCLUDED.overview = '' OR seasons.overview <> ''
				THEN seasons.overview ELSE EXCLUDED.overview END
		 RETURNING id`,
		titleID, seasonNumber, name, overview).Scan(&id)
	return id, err
}

// EpisodeIDsWithMedia returns the title's episode ids that have a video file
// attached - import must not overwrite their metadata.
func (s *Store) EpisodeIDsWithMedia(ctx context.Context, titleID string) (map[string]bool, error) {
	rows, err := s.db.Query(ctx,
		`SELECT DISTINCT mf.episode_id FROM media_files mf
		 JOIN episodes e ON e.id = mf.episode_id
		 JOIN seasons se ON se.id = e.season_id
		 WHERE se.title_id = $1`, titleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// ImportEpisodeMeta upserts an episode from TMDB. With fillOnlyEmpty (episodes
// that have a media file) existing values win; otherwise TMDB wins, though an
// empty TMDB value never clears an existing one. Reports whether a row was
// inserted.
func (s *Store) ImportEpisodeMeta(ctx context.Context, seasonID string, episodeNumber int,
	name, overview string, airDate *time.Time, runtimeMinutes *int, fillOnlyEmpty bool) (id string, created bool, err error) {
	err = s.db.QueryRow(ctx,
		`INSERT INTO episodes (season_id, episode_number, name, overview, air_date, runtime_minutes)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (season_id, episode_number) DO UPDATE SET
			name = CASE WHEN EXCLUDED.name = '' OR ($7 AND episodes.name <> '')
				THEN episodes.name ELSE EXCLUDED.name END,
			overview = CASE WHEN EXCLUDED.overview = '' OR ($7 AND episodes.overview <> '')
				THEN episodes.overview ELSE EXCLUDED.overview END,
			air_date = CASE WHEN EXCLUDED.air_date IS NULL OR ($7 AND episodes.air_date IS NOT NULL)
				THEN episodes.air_date ELSE EXCLUDED.air_date END,
			runtime_minutes = CASE WHEN EXCLUDED.runtime_minutes IS NULL OR ($7 AND episodes.runtime_minutes IS NOT NULL)
				THEN episodes.runtime_minutes ELSE EXCLUDED.runtime_minutes END
		 RETURNING id, (xmax = 0)`,
		seasonID, episodeNumber, name, overview, airDate, runtimeMinutes, fillOnlyEmpty).Scan(&id, &created)
	return id, created, err
}

func (s *Store) DeleteEpisode(ctx context.Context, id string) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM episodes WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return db.ErrNotFound
	}
	return nil
}
