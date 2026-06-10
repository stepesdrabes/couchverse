package store

import (
	"context"
)

type LibraryUsage struct {
	LibraryID int64  `json:"libraryId"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Bytes     int64  `json:"bytes"`
}

func (s *Store) LibraryUsage(ctx context.Context) ([]LibraryUsage, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT l.id, l.name, l.kind, COALESCE(sum(mf.size_bytes), 0)
		 FROM libraries l
		 LEFT JOIN media_files mf ON mf.library_id = l.id
		 GROUP BY l.id
		 ORDER BY l.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	usage := []LibraryUsage{}
	for rows.Next() {
		var u LibraryUsage
		if err := rows.Scan(&u.LibraryID, &u.Name, &u.Kind, &u.Bytes); err != nil {
			return nil, err
		}
		usage = append(usage, u)
	}
	return usage, rows.Err()
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
	err := s.pool.QueryRow(ctx, `
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

// MediaFileIDsForTitles powers the bulk re-scan action.
func (s *Store) MediaFileIDsForTitles(ctx context.Context, titleIDs []int64) ([]int64, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id FROM media_files
		 WHERE title_id = ANY($1)
			OR episode_id IN (
				SELECT e.id FROM episodes e
				JOIN seasons se ON se.id = e.season_id
				WHERE se.title_id = ANY($1))`, titleIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
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
	rows, err := s.pool.Query(ctx,
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
	tx, err := s.pool.Begin(ctx)
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
