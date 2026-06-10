package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"couchverse/internal/httpx"
)

type MediaFile struct {
	ID              string          `json:"id"`
	LibraryID       int64           `json:"libraryId"`
	TitleID         *string         `json:"titleId"`
	EpisodeID       *string         `json:"episodeId"`
	TrackID         *string         `json:"trackId"`
	Path            string          `json:"path"`
	SizeBytes       int64           `json:"sizeBytes"`
	Container       string          `json:"container"`
	VideoCodec      string          `json:"videoCodec"`
	AudioCodec      string          `json:"audioCodec"`
	Width           int             `json:"width"`
	Height          int             `json:"height"`
	DurationSeconds float64         `json:"durationSeconds"`
	Bitrate         int64           `json:"bitrate"`
	Channels        int             `json:"channels"`
	SampleRate      int             `json:"sampleRate"`
	VideoRange      string          `json:"videoRange"`
	DirectPlay      bool            `json:"directPlay"`
	Probe           json.RawMessage `json:"-"`
	FileMtime       *time.Time      `json:"fileMtime"`
	ScannedAt       *time.Time      `json:"scannedAt"`
	SourceDeletedAt *time.Time      `json:"sourceDeletedAt"`
	CreatedAt       time.Time       `json:"createdAt"`
}

const mediaFileCols = `id, library_id, title_id, episode_id, track_id, path, size_bytes, container,
	video_codec, audio_codec, width, height, duration_seconds, bitrate, channels, sample_rate,
	video_range, direct_play, probe, file_mtime, scanned_at, source_deleted_at, created_at`

func scanMediaFile(row pgx.Row) (*MediaFile, error) {
	var m MediaFile
	err := row.Scan(&m.ID, &m.LibraryID, &m.TitleID, &m.EpisodeID, &m.TrackID, &m.Path, &m.SizeBytes,
		&m.Container, &m.VideoCodec, &m.AudioCodec, &m.Width, &m.Height, &m.DurationSeconds,
		&m.Bitrate, &m.Channels, &m.SampleRate, &m.VideoRange, &m.DirectPlay, &m.Probe,
		&m.FileMtime, &m.ScannedAt, &m.SourceDeletedAt, &m.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (s *Store) MediaFileByID(ctx context.Context, id string) (*MediaFile, error) {
	return scanMediaFile(s.pool.QueryRow(ctx,
		`SELECT `+mediaFileCols+` FROM media_files WHERE id = $1`, id))
}

// FileStub is the scanner's view of an on-disk file row.
type FileStub struct {
	ID        string
	SizeBytes int64
	FileMtime *time.Time
}

func (s *Store) MediaFileStubsByLibrary(ctx context.Context, libraryID int64) (map[string]FileStub, error) {
	// rows whose source was cleaned up after transcoding must stay invisible to
	// the scanner, or it treats the missing file as vanished and deletes the row
	rows, err := s.pool.Query(ctx,
		`SELECT path, id, size_bytes, file_mtime FROM media_files
		 WHERE library_id = $1 AND source_deleted_at IS NULL`, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]FileStub{}
	for rows.Next() {
		var path string
		var stub FileStub
		if err := rows.Scan(&path, &stub.ID, &stub.SizeBytes, &stub.FileMtime); err != nil {
			return nil, err
		}
		out[path] = stub
	}
	return out, rows.Err()
}

// UpsertMediaFileStub registers a discovered file, resetting probe data when
// the file changed on disk. Returns the row id.
func (s *Store) UpsertMediaFileStub(ctx context.Context, libraryID int64, path string, size int64, mtime time.Time) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx,
		`INSERT INTO media_files (library_id, path, size_bytes, file_mtime)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (library_id, path) DO UPDATE
			SET size_bytes = EXCLUDED.size_bytes, file_mtime = EXCLUDED.file_mtime,
				scanned_at = NULL, source_deleted_at = NULL
		 RETURNING id`,
		libraryID, path, size, mtime).Scan(&id)
	return id, err
}

func (s *Store) DeleteMediaFile(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM media_files WHERE id = $1`, id)
	return err
}

// MarkSourceDeleted records that the original file was removed after
// transcoding; playback must use HLS variants from now on.
func (s *Store) MarkSourceDeleted(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE media_files SET source_deleted_at = now(), direct_play = false, size_bytes = 0
		 WHERE id = $1`, id)
	return err
}

type ProbeUpdate struct {
	Container       string
	VideoCodec      string
	AudioCodec      string
	Width, Height   int
	DurationSeconds float64
	Bitrate         int64
	Channels        int
	SampleRate      int
	VideoRange      string
	DirectPlay      bool
	Probe           json.RawMessage
	TitleID         *string
	EpisodeID       *string
	TrackID         *string
}

func (s *Store) ApplyProbe(ctx context.Context, id string, up ProbeUpdate) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE media_files SET
			container = $2, video_codec = $3, audio_codec = $4, width = $5, height = $6,
			duration_seconds = $7, bitrate = $8, channels = $9, sample_rate = $10,
			video_range = $11, direct_play = $12, probe = $13,
			title_id = $14, episode_id = $15, track_id = $16,
			scanned_at = now()
		 WHERE id = $1`,
		id, up.Container, up.VideoCodec, up.AudioCodec, up.Width, up.Height,
		up.DurationSeconds, up.Bitrate, up.Channels, up.SampleRate,
		up.VideoRange, up.DirectPlay, up.Probe, up.TitleID, up.EpisodeID, up.TrackID)
	return err
}

func (s *Store) MediaFilesForTitle(ctx context.Context, titleID string) ([]MediaFile, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+mediaFileCols+` FROM media_files
		 WHERE title_id = $1
			OR episode_id IN (SELECT e.id FROM episodes e JOIN seasons se ON se.id = e.season_id WHERE se.title_id = $1)
		 ORDER BY path`, titleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	files := []MediaFile{}
	for rows.Next() {
		m, err := scanMediaFile(rows)
		if err != nil {
			return nil, err
		}
		files = append(files, *m)
	}
	return files, rows.Err()
}
