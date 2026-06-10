package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"couchverse/internal/httpx"
)

type Artwork struct {
	ID        int64     `json:"id"`
	OwnerKind string    `json:"ownerKind"`
	OwnerID   int64     `json:"ownerId"`
	Kind      string    `json:"kind"`
	Path      string    `json:"path"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"createdAt"`
}

const artworkCols = `id, owner_kind, owner_id, kind, path, width, height, source, created_at`

func scanArtwork(row pgx.Row) (*Artwork, error) {
	var a Artwork
	err := row.Scan(&a.ID, &a.OwnerKind, &a.OwnerID, &a.Kind, &a.Path, &a.Width, &a.Height, &a.Source, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// SetArtwork upserts one artwork slot (e.g. a title's poster).
func (s *Store) SetArtwork(ctx context.Context, ownerKind string, ownerID int64, kind, path string, w, h int, source string) (*Artwork, error) {
	return scanArtwork(s.pool.QueryRow(ctx,
		`INSERT INTO artwork (owner_kind, owner_id, kind, path, width, height, source)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (owner_kind, owner_id, kind) DO UPDATE
			SET path = EXCLUDED.path, width = EXCLUDED.width, height = EXCLUDED.height,
				source = EXCLUDED.source, created_at = now()
		 RETURNING `+artworkCols,
		ownerKind, ownerID, kind, path, w, h, source))
}

func (s *Store) ArtworkByID(ctx context.Context, id int64) (*Artwork, error) {
	return scanArtwork(s.pool.QueryRow(ctx,
		`SELECT `+artworkCols+` FROM artwork WHERE id = $1`, id))
}

func (s *Store) ArtworkFor(ctx context.Context, ownerKind string, ownerID int64) ([]Artwork, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+artworkCols+` FROM artwork WHERE owner_kind = $1 AND owner_id = $2`, ownerKind, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []Artwork{}
	for rows.Next() {
		a, err := scanArtwork(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *a)
	}
	return items, rows.Err()
}

func (s *Store) DeleteArtwork(ctx context.Context, id int64) (*Artwork, error) {
	return scanArtwork(s.pool.QueryRow(ctx,
		`DELETE FROM artwork WHERE id = $1 RETURNING `+artworkCols, id))
}

// DeleteArtworkForOwner removes all artwork rows of an owner, returning them
// so callers can clean up files.
func (s *Store) DeleteArtworkForOwner(ctx context.Context, ownerKind string, ownerID int64) ([]Artwork, error) {
	rows, err := s.pool.Query(ctx,
		`DELETE FROM artwork WHERE owner_kind = $1 AND owner_id = $2 RETURNING `+artworkCols,
		ownerKind, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []Artwork{}
	for rows.Next() {
		a, err := scanArtwork(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *a)
	}
	return items, rows.Err()
}
