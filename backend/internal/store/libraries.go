package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"couchverse/internal/db"
)

type Library struct {
	ID            int64      `json:"id"`
	Name          string     `json:"name"`
	Kind          string     `json:"kind"`
	Path          string     `json:"path"`
	Managed       bool       `json:"managed"`
	LastScannedAt *time.Time `json:"lastScannedAt"`
}

func scanLibrary(row pgx.Row) (*Library, error) {
	var l Library
	err := row.Scan(&l.ID, &l.Name, &l.Kind, &l.Path, &l.Managed, &l.LastScannedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, db.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &l, nil
}

const libraryCols = `id, name, kind, path, managed, last_scanned_at`

func (s *Store) ListLibraries(ctx context.Context) ([]Library, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+libraryCols+` FROM libraries ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	libs := []Library{}
	for rows.Next() {
		l, err := scanLibrary(rows)
		if err != nil {
			return nil, err
		}
		libs = append(libs, *l)
	}
	return libs, rows.Err()
}

func (s *Store) LibraryByID(ctx context.Context, id int64) (*Library, error) {
	return scanLibrary(s.pool.QueryRow(ctx,
		`SELECT `+libraryCols+` FROM libraries WHERE id = $1`, id))
}

func (s *Store) CreateLibrary(ctx context.Context, name, kind, path string, managed bool) (*Library, error) {
	return scanLibrary(s.pool.QueryRow(ctx,
		`INSERT INTO libraries (name, kind, path, managed) VALUES ($1, $2, $3, $4)
		 RETURNING `+libraryCols, name, kind, path, managed))
}

// EnsureLibrary creates the library if its path is not registered yet.
func (s *Store) EnsureLibrary(ctx context.Context, name, kind, path string, managed bool) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO libraries (name, kind, path, managed) VALUES ($1, $2, $3, $4)
		 ON CONFLICT (path) DO NOTHING`, name, kind, path, managed)
	return err
}

func (s *Store) DeleteLibrary(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM libraries WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return db.ErrNotFound
	}
	return nil
}

func (s *Store) TouchLibraryScanned(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx, `UPDATE libraries SET last_scanned_at = now() WHERE id = $1`, id)
	return err
}

// ManagedLibraryByKind returns the managed library uploads land in.
func (s *Store) ManagedLibraryByKind(ctx context.Context, kind string) (*Library, error) {
	return scanLibrary(s.pool.QueryRow(ctx,
		`SELECT `+libraryCols+` FROM libraries WHERE kind = $1 AND managed ORDER BY id LIMIT 1`, kind))
}
