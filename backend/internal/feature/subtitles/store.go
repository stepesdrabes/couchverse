package subtitles

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"couchverse/internal/media"
)

// Store owns the subtitles SQL over the shared pool.
type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

func (s *Store) CreateSubtitle(ctx context.Context, mediaFileID string, lang, label, source string, forced bool, path string) (*media.Subtitle, error) {
	return media.ScanSubtitle(s.db.QueryRow(ctx,
		`INSERT INTO subtitles (media_file_id, lang, label, source, forced, path)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING `+media.SubtitleCols,
		mediaFileID, lang, label, source, forced, path))
}

func (s *Store) SubtitleByID(ctx context.Context, id string) (*media.Subtitle, error) {
	return media.ScanSubtitle(s.db.QueryRow(ctx,
		`SELECT `+media.SubtitleCols+` FROM subtitles WHERE id = $1`, id))
}

func (s *Store) SubtitlesForMediaFile(ctx context.Context, mediaFileID string) ([]media.Subtitle, error) {
	rows, err := s.db.Query(ctx,
		`SELECT `+media.SubtitleCols+` FROM subtitles WHERE media_file_id = $1 ORDER BY lang, id`, mediaFileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	subs := []media.Subtitle{}
	for rows.Next() {
		sub, err := media.ScanSubtitle(rows)
		if err != nil {
			return nil, err
		}
		subs = append(subs, *sub)
	}
	return subs, rows.Err()
}

func (s *Store) UpdateSubtitlePath(ctx context.Context, id string, path string) (*media.Subtitle, error) {
	return media.ScanSubtitle(s.db.QueryRow(ctx,
		`UPDATE subtitles SET path = $2 WHERE id = $1 RETURNING `+media.SubtitleCols, id, path))
}

func (s *Store) DeleteSubtitle(ctx context.Context, id string) (*media.Subtitle, error) {
	return media.ScanSubtitle(s.db.QueryRow(ctx,
		`DELETE FROM subtitles WHERE id = $1 RETURNING `+media.SubtitleCols, id))
}

// DeleteEmbeddedSubtitles clears previously extracted rows before re-extraction.
func (s *Store) DeleteEmbeddedSubtitles(ctx context.Context, mediaFileID string) error {
	_, err := s.db.Exec(ctx,
		`DELETE FROM subtitles WHERE media_file_id = $1 AND source = 'embedded'`, mediaFileID)
	return err
}
