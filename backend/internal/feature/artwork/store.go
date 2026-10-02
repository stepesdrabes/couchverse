package artwork

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"couchverse/internal/db"
)

// Store owns the artwork rows; image files live under DataDir/artwork.
type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

// Artwork is one image slot of an owner (a title, season, episode or user). A
// slot is the owner, the kind and the language, so a title holds one logo per
// content language.
type Artwork struct {
	ID        string    `json:"id"`
	OwnerKind string    `json:"ownerKind" enum:"title,season,episode,user"`
	OwnerID   string    `json:"ownerId" doc:"The owner's uuid, or the numeric user id for user artwork."`
	Kind      string    `json:"kind" enum:"poster,backdrop,thumb,avatar,banner,logo" doc:"A title's poster, backdrop or logo (a transparent PNG wordmark, one per content language), an episode's thumb, a user's avatar or banner."`
	Lang      *string   `json:"lang" doc:"Content language (ISO 639-1) of a title logo; null for art not tied to a language."`
	Path      string    `json:"path" doc:"Location under the server's data directory."`
	Width     int       `json:"width" doc:"0 when not measured."`
	Height    int       `json:"height" doc:"0 when not measured."`
	Source    string    `json:"source" enum:"tmdb,uploaded,embedded"`
	Accent    string    `json:"accent,omitempty" doc:"Dominant vibrant colour as #rrggbb; - when the image yielded none."`
	CreatedAt time.Time `json:"createdAt"`
}

const artworkCols = `id, owner_kind, owner_id, kind, lang, path, width, height, source, accent, created_at`

func scanArtwork(row pgx.Row) (*Artwork, error) {
	var a Artwork
	err := row.Scan(&a.ID, &a.OwnerKind, &a.OwnerID, &a.Kind, &a.Lang, &a.Path, &a.Width, &a.Height, &a.Source, &a.Accent, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, db.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// SetArtwork upserts one artwork slot (e.g. a title's poster, or its Czech
// logo); an empty lang is the slot not tied to a language.
func (s *Store) SetArtwork(ctx context.Context, ownerKind string, ownerID string, kind, lang, path string, w, h int, source, accent string) (*Artwork, error) {
	return scanArtwork(s.db.QueryRow(ctx,
		`INSERT INTO artwork (owner_kind, owner_id, kind, lang, path, width, height, source, accent)
		 VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6, $7, $8, $9)
		 ON CONFLICT (owner_kind, owner_id, kind, lang) DO UPDATE
			SET path = EXCLUDED.path, width = EXCLUDED.width, height = EXCLUDED.height,
				source = EXCLUDED.source, accent = EXCLUDED.accent, created_at = now()
		 RETURNING `+artworkCols,
		ownerKind, ownerID, kind, lang, path, w, h, source, accent))
}

// SetAccent stores a freshly computed accent for an existing artwork row
// (used by the background backfill).
func (s *Store) SetAccent(ctx context.Context, id, accent string) error {
	_, err := s.db.Exec(ctx, `UPDATE artwork SET accent = $2 WHERE id = $1`, id, accent)
	return err
}

// ArtworkMissingAccent returns up to limit artwork ids+paths that have no
// accent yet, for the startup backfill.
func (s *Store) ArtworkMissingAccent(ctx context.Context, limit int) ([]Artwork, error) {
	return collect(s.db.Query(ctx,
		`SELECT `+artworkCols+` FROM artwork WHERE accent = '' ORDER BY created_at LIMIT $1`, limit))
}

func (s *Store) ArtworkByID(ctx context.Context, id string) (*Artwork, error) {
	return scanArtwork(s.db.QueryRow(ctx,
		`SELECT `+artworkCols+` FROM artwork WHERE id = $1`, id))
}

func (s *Store) ArtworkFor(ctx context.Context, ownerKind string, ownerID string) ([]Artwork, error) {
	return collect(s.db.Query(ctx,
		`SELECT `+artworkCols+` FROM artwork WHERE owner_kind = $1 AND owner_id = $2
		 ORDER BY kind, lang NULLS FIRST`, ownerKind, ownerID))
}

func (s *Store) DeleteArtwork(ctx context.Context, id string) (*Artwork, error) {
	return scanArtwork(s.db.QueryRow(ctx,
		`DELETE FROM artwork WHERE id = $1 RETURNING `+artworkCols, id))
}

// DeleteArtworkForOwner removes all artwork rows of an owner, returning them
// so callers can clean up files.
func (s *Store) DeleteArtworkForOwner(ctx context.Context, ownerKind string, ownerID string) ([]Artwork, error) {
	return collect(s.db.Query(ctx,
		`DELETE FROM artwork WHERE owner_kind = $1 AND owner_id = $2 RETURNING `+artworkCols,
		ownerKind, ownerID))
}

// DeleteArtworkForLang removes an owner's artwork in one language (a title's
// logo when that content language goes), returning the rows.
func (s *Store) DeleteArtworkForLang(ctx context.Context, ownerKind string, ownerID string, lang string) ([]Artwork, error) {
	return collect(s.db.Query(ctx,
		`DELETE FROM artwork WHERE owner_kind = $1 AND owner_id = $2 AND lang = $3 RETURNING `+artworkCols,
		ownerKind, ownerID, lang))
}

func collect(rows pgx.Rows, err error) ([]Artwork, error) {
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
