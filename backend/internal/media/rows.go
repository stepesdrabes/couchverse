package media

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"couchverse/internal/db"
)

// MediaFile and Subtitle are the shared media-asset row types. The library
// feature owns the media_files SQL and the subtitles feature owns the
// subtitles SQL, but catalog and playback read these rows too - the types
// live here so the feature import graph stays acyclic.

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

const MediaFileCols = `id, library_id, title_id, episode_id, track_id, path, size_bytes, container,
	video_codec, audio_codec, width, height, duration_seconds, bitrate, channels, sample_rate,
	video_range, direct_play, probe, file_mtime, scanned_at, source_deleted_at, created_at`

func ScanMediaFile(row pgx.Row) (*MediaFile, error) {
	var m MediaFile
	err := row.Scan(&m.ID, &m.LibraryID, &m.TitleID, &m.EpisodeID, &m.TrackID, &m.Path, &m.SizeBytes,
		&m.Container, &m.VideoCodec, &m.AudioCodec, &m.Width, &m.Height, &m.DurationSeconds,
		&m.Bitrate, &m.Channels, &m.SampleRate, &m.VideoRange, &m.DirectPlay, &m.Probe,
		&m.FileMtime, &m.ScannedAt, &m.SourceDeletedAt, &m.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, db.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

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

const SubtitleCols = `id, media_file_id, lang, label, source, forced, path, created_at`

func ScanSubtitle(row pgx.Row) (*Subtitle, error) {
	var s Subtitle
	err := row.Scan(&s.ID, &s.MediaFileID, &s.Lang, &s.Label, &s.Source, &s.Forced, &s.Path, &s.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, db.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}
