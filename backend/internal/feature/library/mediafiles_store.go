package library

import (
	"context"
	"encoding/json"
	"time"

	"couchverse/internal/media"
)

func (s *Store) MediaFileByID(ctx context.Context, id string) (*media.MediaFile, error) {
	return media.ScanMediaFile(s.db.QueryRow(ctx,
		`SELECT `+media.MediaFileCols+` FROM media_files WHERE id = $1`, id))
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
	rows, err := s.db.Query(ctx,
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
	err := s.db.QueryRow(ctx,
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
	_, err := s.db.Exec(ctx, `DELETE FROM media_files WHERE id = $1`, id)
	return err
}

// SetMediaFileAudio tags a file's audio language and role for model-B
// multi-language audio ('primary' full file vs 'audio_alt' sibling).
func (s *Store) SetMediaFileAudio(ctx context.Context, id, lang, role string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE media_files SET audio_lang = $2, audio_role = $3 WHERE id = $1`, id, lang, role)
	return err
}

// MarkSourceDeleted records that the original file was removed after
// transcoding; playback must use HLS variants from now on.
func (s *Store) MarkSourceDeleted(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx,
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
	_, err := s.db.Exec(ctx,
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
