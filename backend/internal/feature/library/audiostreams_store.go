package library

import (
	"context"

	"couchverse/internal/media"
)

// ReplaceAudioStreams rewrites a media file's embedded-audio inventory from a
// fresh probe (clear + reinsert), mirroring how subtitles are refreshed.
func (s *Store) ReplaceAudioStreams(ctx context.Context, mediaFileID string, streams []media.AudioStream) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM audio_streams WHERE media_file_id = $1`, mediaFileID); err != nil {
		return err
	}
	for _, a := range streams {
		lang := a.Lang
		if lang == "" {
			lang = "und"
		}
		label := a.Title
		if label == "" {
			label = lang
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO audio_streams (media_file_id, stream_index, codec, lang, label, channels, is_default)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			mediaFileID, a.Index, a.Codec, lang, label, a.Channels, a.Default); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// AudioStreamsForFile returns a media file's embedded audio tracks, ordered by
// stream index (label is returned in the Title field).
func (s *Store) AudioStreamsForFile(ctx context.Context, mediaFileID string) ([]media.AudioStream, error) {
	rows, err := s.db.Query(ctx,
		`SELECT stream_index, codec, lang, label, channels, is_default
		 FROM audio_streams WHERE media_file_id = $1 ORDER BY stream_index`, mediaFileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []media.AudioStream{}
	for rows.Next() {
		var a media.AudioStream
		if err := rows.Scan(&a.Index, &a.Codec, &a.Lang, &a.Title, &a.Channels, &a.Default); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
