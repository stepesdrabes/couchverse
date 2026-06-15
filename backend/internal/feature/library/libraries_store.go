package library

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

func (s *Store) LibraryByID(ctx context.Context, id int64) (*Library, error) {
	return scanLibrary(s.db.QueryRow(ctx,
		`SELECT `+libraryCols+` FROM libraries WHERE id = $1`, id))
}

// EnsureLibrary creates the library if its path is not registered yet.
func (s *Store) EnsureLibrary(ctx context.Context, name, kind, path string, managed bool) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO libraries (name, kind, path, managed) VALUES ($1, $2, $3, $4)
		 ON CONFLICT (path) DO NOTHING`, name, kind, path, managed)
	return err
}

// ManagedLibraryByKind returns the managed library uploads land in.
func (s *Store) ManagedLibraryByKind(ctx context.Context, kind string) (*Library, error) {
	return scanLibrary(s.db.QueryRow(ctx,
		`SELECT `+libraryCols+` FROM libraries WHERE kind = $1 AND managed ORDER BY id LIMIT 1`, kind))
}
