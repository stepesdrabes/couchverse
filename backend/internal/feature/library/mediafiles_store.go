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
	Video           media.VideoStream
	DirectPlay      bool
	Probe           json.RawMessage
	TitleID         *string
	EpisodeID       *string
}

// ProbeUpdateFrom carries a probe result into the columns ApplyProbe writes;
// the caller sets the catalog assignment.
func ProbeUpdateFrom(res *media.ProbeResult) ProbeUpdate {
	return ProbeUpdate{
		Container:       res.Container,
		VideoCodec:      res.VideoCodec,
		AudioCodec:      res.AudioCodec,
		Width:           res.Width,
		Height:          res.Height,
		DurationSeconds: res.DurationSeconds,
		Bitrate:         res.Bitrate,
		Channels:        res.Channels,
		SampleRate:      res.SampleRate,
		VideoRange:      res.VideoRange,
		Video:           res.Video,
		DirectPlay:      media.DirectPlay(res),
		Probe:           res.Raw,
	}
}

func (s *Store) ApplyProbe(ctx context.Context, id string, up ProbeUpdate) error {
	v := up.Video
	_, err := s.db.Exec(ctx,
		`UPDATE media_files SET
			container = $2, video_codec = $3, audio_codec = $4, width = $5, height = $6,
			duration_seconds = $7, bitrate = $8, channels = $9, sample_rate = $10,
			video_range = $11, direct_play = $12, probe = $13,
			title_id = $14, episode_id = $15,
			video_codec_tag = $16, video_profile = $17, video_level = $18, bit_depth = $19,
			frame_rate = $20, hdr_format = $21, dovi_profile = $22, dovi_compatibility = $23,
			probe_version = $24, scanned_at = now()
		 WHERE id = $1`,
		id, up.Container, up.VideoCodec, up.AudioCodec, up.Width, up.Height,
		up.DurationSeconds, up.Bitrate, up.Channels, up.SampleRate,
		up.VideoRange, up.DirectPlay, up.Probe, up.TitleID, up.EpisodeID,
		v.CodecTag, v.Profile, v.Level, v.BitDepth, v.FrameRate, hdrFormat(v.HDR),
		v.DoviProfile, v.DoviCompatibility, media.ProbeVersion)
	return err
}

func hdrFormat(hdr string) string {
	if hdr == "" {
		return media.HDRNone
	}
	return hdr
}

// SetProbeVersion records which prober last handled a file.
func (s *Store) SetProbeVersion(ctx context.Context, id string, version int) error {
	_, err := s.db.Exec(ctx, `UPDATE media_files SET probe_version = $2 WHERE id = $1`, id, version)
	return err
}

// StaleProbes lists video files an older prober read, oldest first.
func (s *Store) StaleProbes(ctx context.Context, limit int) ([]media.MediaFile, error) {
	rows, err := s.db.Query(ctx,
		`SELECT `+media.MediaFileCols+` FROM media_files
		 WHERE probe_version < $1 AND scanned_at IS NOT NULL AND video_codec <> ''
		 ORDER BY created_at LIMIT $2`, media.ProbeVersion, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []media.MediaFile{}
	for rows.Next() {
		m, err := media.ScanMediaFile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *m)
	}
	return out, rows.Err()
}
