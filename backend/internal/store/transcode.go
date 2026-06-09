package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"couchverse/internal/httpx"
)

type TranscodeVariant struct {
	ID           int64      `json:"id"`
	MediaFileID  int64      `json:"mediaFileId"`
	Name         string     `json:"name"`
	Width        int        `json:"width"`
	Height       int        `json:"height"`
	VideoBitrate int64      `json:"videoBitrate"`
	AudioBitrate int64      `json:"audioBitrate"`
	Mode         string     `json:"mode"`
	Status       string     `json:"status"`
	PlaylistPath string     `json:"-"`
	CreatedAt    time.Time  `json:"createdAt"`
	CompletedAt  *time.Time `json:"completedAt"`
}

const variantCols = `id, media_file_id, name, width, height, video_bitrate, audio_bitrate,
	mode, status, playlist_path, created_at, completed_at`

func scanVariant(row pgx.Row) (*TranscodeVariant, error) {
	var v TranscodeVariant
	err := row.Scan(&v.ID, &v.MediaFileID, &v.Name, &v.Width, &v.Height, &v.VideoBitrate,
		&v.AudioBitrate, &v.Mode, &v.Status, &v.PlaylistPath, &v.CreatedAt, &v.CompletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// UpsertVariant registers/resets a variant slot before its job runs.
func (s *Store) UpsertVariant(ctx context.Context, mediaFileID int64, name string, height int, videoBitrate, audioBitrate int64, mode string) (*TranscodeVariant, error) {
	return scanVariant(s.pool.QueryRow(ctx,
		`INSERT INTO transcode_variants (media_file_id, name, height, video_bitrate, audio_bitrate, mode)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (media_file_id, name) DO UPDATE
			SET status = 'queued', mode = EXCLUDED.mode, height = EXCLUDED.height,
				video_bitrate = EXCLUDED.video_bitrate, audio_bitrate = EXCLUDED.audio_bitrate,
				playlist_path = '', completed_at = NULL
		 RETURNING `+variantCols,
		mediaFileID, name, height, videoBitrate, audioBitrate, mode))
}

func (s *Store) SetVariantStatus(ctx context.Context, id int64, status, playlistPath string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE transcode_variants SET status = $2, playlist_path = $3,
			completed_at = CASE WHEN $2 = 'ready' THEN now() ELSE NULL END
		 WHERE id = $1`, id, status, playlistPath)
	return err
}

func (s *Store) VariantsForMediaFile(ctx context.Context, mediaFileID int64) ([]TranscodeVariant, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+variantCols+` FROM transcode_variants
		 WHERE media_file_id = $1 ORDER BY height DESC`, mediaFileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	variants := []TranscodeVariant{}
	for rows.Next() {
		v, err := scanVariant(rows)
		if err != nil {
			return nil, err
		}
		variants = append(variants, *v)
	}
	return variants, rows.Err()
}

func (s *Store) VariantByID(ctx context.Context, id int64) (*TranscodeVariant, error) {
	return scanVariant(s.pool.QueryRow(ctx,
		`SELECT `+variantCols+` FROM transcode_variants WHERE id = $1`, id))
}

func (s *Store) DeleteVariant(ctx context.Context, id int64) (*TranscodeVariant, error) {
	return scanVariant(s.pool.QueryRow(ctx,
		`DELETE FROM transcode_variants WHERE id = $1 RETURNING `+variantCols, id))
}
