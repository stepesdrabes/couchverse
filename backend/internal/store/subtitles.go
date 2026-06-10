package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"couchverse/internal/httpx"
)

type Subtitle struct {
	ID          string    `json:"id"`
	MediaFileID string    `json:"mediaFileId"`
	Lang        string    `json:"lang"`
	Label       string    `json:"label"`
	Source      string    `json:"source"`
	Forced      bool      `json:"forced"`
	Path        string    `json:"-"`
	CreatedAt   time.Time `json:"createdAt"`
}

const subtitleCols = `id, media_file_id, lang, label, source, forced, path, created_at`

func scanSubtitle(row pgx.Row) (*Subtitle, error) {
	var s Subtitle
	err := row.Scan(&s.ID, &s.MediaFileID, &s.Lang, &s.Label, &s.Source, &s.Forced, &s.Path, &s.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (s *Store) CreateSubtitle(ctx context.Context, mediaFileID string, lang, label, source string, forced bool, path string) (*Subtitle, error) {
	return scanSubtitle(s.pool.QueryRow(ctx,
		`INSERT INTO subtitles (media_file_id, lang, label, source, forced, path)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING `+subtitleCols,
		mediaFileID, lang, label, source, forced, path))
}

func (s *Store) SubtitleByID(ctx context.Context, id string) (*Subtitle, error) {
	return scanSubtitle(s.pool.QueryRow(ctx,
		`SELECT `+subtitleCols+` FROM subtitles WHERE id = $1`, id))
}

func (s *Store) SubtitlesForMediaFile(ctx context.Context, mediaFileID string) ([]Subtitle, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+subtitleCols+` FROM subtitles WHERE media_file_id = $1 ORDER BY lang, id`, mediaFileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	subs := []Subtitle{}
	for rows.Next() {
		sub, err := scanSubtitle(rows)
		if err != nil {
			return nil, err
		}
		subs = append(subs, *sub)
	}
	return subs, rows.Err()
}

// SubtitlesForMediaFiles bulk-loads subtitles keyed by media file id.
func (s *Store) SubtitlesForMediaFiles(ctx context.Context, mediaFileIDs []string) (map[string][]Subtitle, error) {
	out := map[string][]Subtitle{}
	if len(mediaFileIDs) == 0 {
		return out, nil
	}
	rows, err := s.pool.Query(ctx,
		`SELECT `+subtitleCols+` FROM subtitles
		 WHERE media_file_id = ANY($1::uuid[]) ORDER BY lang, id`, mediaFileIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		sub, err := scanSubtitle(rows)
		if err != nil {
			return nil, err
		}
		out[sub.MediaFileID] = append(out[sub.MediaFileID], *sub)
	}
	return out, rows.Err()
}

func (s *Store) UpdateSubtitlePath(ctx context.Context, id string, path string) (*Subtitle, error) {
	return scanSubtitle(s.pool.QueryRow(ctx,
		`UPDATE subtitles SET path = $2 WHERE id = $1 RETURNING `+subtitleCols, id, path))
}

func (s *Store) DeleteSubtitle(ctx context.Context, id string) (*Subtitle, error) {
	return scanSubtitle(s.pool.QueryRow(ctx,
		`DELETE FROM subtitles WHERE id = $1 RETURNING `+subtitleCols, id))
}

// DeleteEmbeddedSubtitles clears previously extracted rows before re-extraction.
func (s *Store) DeleteEmbeddedSubtitles(ctx context.Context, mediaFileID string) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM subtitles WHERE media_file_id = $1 AND source = 'embedded'`, mediaFileID)
	return err
}
