// Package system owns server-level concerns: theme, admin settings, storage
// stats, host metrics and the home-rows editor.
package system

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

// MediaUsageByKind sums media file sizes per library kind (movies/series/music),
// collapsing multiple libraries of the same kind into one total.
func (s *Store) MediaUsageByKind(ctx context.Context) (map[string]int64, error) {
	rows, err := s.db.Query(ctx,
		`SELECT l.kind, COALESCE(sum(mf.size_bytes), 0)
		 FROM libraries l
		 LEFT JOIN media_files mf ON mf.library_id = l.id
		 GROUP BY l.kind`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]int64{}
	for rows.Next() {
		var kind string
		var bytes int64
		if err := rows.Scan(&kind, &bytes); err != nil {
			return nil, err
		}
		out[kind] = bytes
	}
	return out, rows.Err()
}

// TranscodeUsage sums the on-disk size of ready HLS variants.
func (s *Store) TranscodeUsage(ctx context.Context) (int64, error) {
	var total int64
	err := s.db.QueryRow(ctx,
		`SELECT COALESCE(sum(size_bytes), 0) FROM transcode_variants WHERE status = 'ready'`).
		Scan(&total)
	return total, err
}

type OverviewCounts struct {
	Movies   int `json:"movies"`
	Series   int `json:"series"`
	Episodes int `json:"episodes"`
	Albums   int `json:"albums"`
	Tracks   int `json:"tracks"`
	Users    int `json:"users"`
}

func (s *Store) Overview(ctx context.Context) (*OverviewCounts, error) {
	var c OverviewCounts
	err := s.db.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM titles WHERE kind = 'movie'),
			(SELECT count(*) FROM titles WHERE kind = 'series'),
			(SELECT count(*) FROM episodes),
			(SELECT count(*) FROM albums),
			(SELECT count(*) FROM tracks),
			(SELECT count(*) FROM users)`).
		Scan(&c.Movies, &c.Series, &c.Episodes, &c.Albums, &c.Tracks, &c.Users)
	return &c, err
}

type LibraryStats struct {
	TotalRuntimeSeconds int64 `json:"totalRuntimeSeconds"`
	Quality             struct {
		UHD int `json:"uhd"`
		FHD int `json:"fhd"`
		HD  int `json:"hd"`
		SD  int `json:"sd"`
	} `json:"quality"`
	HDR             int `json:"hdr"`
	AddedLast30Days int `json:"addedLast30Days"`
}

// LibraryStats summarises the video library: total runtime, a resolution
// breakdown, HDR count and how many titles were added in the last 30 days.
func (s *Store) LibraryStats(ctx context.Context) (*LibraryStats, error) {
	var ls LibraryStats
	err := s.db.QueryRow(ctx, `
		SELECT
			COALESCE(sum(mf.duration_seconds), 0)::bigint,
			count(*) FILTER (WHERE mf.height >= 2160),
			count(*) FILTER (WHERE mf.height >= 1080 AND mf.height < 2160),
			count(*) FILTER (WHERE mf.height >= 720 AND mf.height < 1080),
			count(*) FILTER (WHERE mf.height > 0 AND mf.height < 720),
			count(*) FILTER (WHERE mf.video_range <> 'sdr')
		FROM media_files mf
		JOIN libraries l ON l.id = mf.library_id
		WHERE l.kind IN ('movies', 'series')`).
		Scan(&ls.TotalRuntimeSeconds, &ls.Quality.UHD, &ls.Quality.FHD, &ls.Quality.HD, &ls.Quality.SD, &ls.HDR)
	if err != nil {
		return nil, err
	}
	if err := s.db.QueryRow(ctx,
		`SELECT count(*) FROM titles WHERE added_at > now() - interval '30 days'`).
		Scan(&ls.AddedLast30Days); err != nil {
		return nil, err
	}
	return &ls, nil
}

// ActiveStreamCount counts players that beaconed progress in the last 60s - a
// proxy for "currently streaming". Couch followers/anonymous don't write
// progress, so they are counted separately via the couch hub.
func (s *Store) ActiveStreamCount(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRow(ctx,
		`SELECT count(*) FROM watch_progress WHERE updated_at > now() - interval '60 seconds'`).
		Scan(&n)
	return n, err
}

type HomeRowConfig struct {
	ID       int64  `json:"id"`
	Position int    `json:"position"`
	Kind     string `json:"kind"`
	GenreID  *int64 `json:"genreId"`
	Label    string `json:"label"`
	Enabled  bool   `json:"enabled"`
}

func (s *Store) ListHomeRows(ctx context.Context) ([]HomeRowConfig, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, position, kind, genre_id, label, enabled FROM home_rows ORDER BY position`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	configs := []HomeRowConfig{}
	for rows.Next() {
		var c HomeRowConfig
		if err := rows.Scan(&c.ID, &c.Position, &c.Kind, &c.GenreID, &c.Label, &c.Enabled); err != nil {
			return nil, err
		}
		configs = append(configs, c)
	}
	return configs, rows.Err()
}

// ReplaceHomeRows rewrites the home page row config atomically.
func (s *Store) ReplaceHomeRows(ctx context.Context, configs []HomeRowConfig) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM home_rows`); err != nil {
		return err
	}
	for i, c := range configs {
		if _, err := tx.Exec(ctx,
			`INSERT INTO home_rows (position, kind, genre_id, label, enabled)
			 VALUES ($1, $2, $3, $4, $5)`,
			i+1, c.Kind, c.GenreID, c.Label, c.Enabled); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
