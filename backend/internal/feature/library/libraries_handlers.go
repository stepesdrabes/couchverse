package library

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
)

type AdminLibraries struct {
	store   *Store
	dataDir string
}

func NewAdminLibraries(st *Store, dataDir string) *AdminLibraries {
	return &AdminLibraries{store: st, dataDir: dataDir}
}

type idInput struct {
	ID string `path:"id" format:"uuid"`
}

// MediaFileAudio tags a media file's audio. Both fields are written: an absent
// audioLang clears the language.
type MediaFileAudio struct {
	AudioLang string `json:"audioLang" required:"false" doc:"Audio language code; empty for an untagged file."`
	AudioRole string `json:"audioRole" required:"false" enum:"primary,audio_alt" default:"primary" doc:"audio_alt marks a separate-language sibling of the title's or episode's primary file."`
}

type setMediaFileAudioInput struct {
	ID   string `path:"id" format:"uuid"`
	Body MediaFileAudio
}

// SetMediaFileAudio tags a media file with an audio language and role so it can
// act as an alternate-audio sibling (model B).
func (h *AdminLibraries) SetMediaFileAudio(ctx context.Context, in *setMediaFileAudioInput) (*struct{}, error) {
	return nil, h.store.SetMediaFileAudio(ctx, in.ID, in.Body.AudioLang, in.Body.AudioRole)
}

// DeleteMediaFile removes a media file: its source on disk, its HLS/frame caches
// and subtitle files, then the row (which cascades the subtitle and transcode
// variant rows). Disk removal is best-effort - the hourly cleanup sweeps any
// leftovers - so a missing file never blocks the delete.
func (h *AdminLibraries) DeleteMediaFile(ctx context.Context, in *idInput) (*struct{}, error) {
	id := in.ID
	mf, err := h.store.MediaFileByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if mf.SourceDeletedAt == nil {
		if lib, lerr := h.store.LibraryByID(ctx, mf.LibraryID); lerr == nil {
			src := filepath.Join(lib.Path, mf.Path)
			if err := os.Remove(src); err != nil && !errors.Is(err, fs.ErrNotExist) {
				slog.Warn("delete media file: remove source", "path", src, "err", err)
			}
		} else {
			slog.Warn("delete media file: resolve library", "mediaFileId", id, "err", lerr)
		}
	}
	for _, dir := range []string{
		filepath.Join(h.dataDir, "cache", "hls", id),
		filepath.Join(h.dataDir, "cache", "frames", id),
		filepath.Join(h.dataDir, "subtitles", id),
	} {
		if err := os.RemoveAll(dir); err != nil {
			slog.Warn("delete media file: remove dir", "dir", dir, "err", err)
		}
	}
	return nil, h.store.DeleteMediaFile(ctx, id)
}
