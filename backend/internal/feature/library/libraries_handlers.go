package library

import (
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"couchverse/internal/httpx"
)

type AdminLibraries struct {
	store   *Store
	dataDir string
}

func NewAdminLibraries(st *Store, dataDir string) *AdminLibraries {
	return &AdminLibraries{store: st, dataDir: dataDir}
}

// SetMediaFileAudio tags a media file with an audio language and role so it can
// act as an alternate-audio sibling (model B).
func (h *AdminLibraries) SetMediaFileAudio(w http.ResponseWriter, r *http.Request) {
	id := httpx.UUID(r, "id")
	if id == "" {
		httpx.NotFound(w)
		return
	}
	var req struct {
		AudioLang string `json:"audioLang"`
		AudioRole string `json:"audioRole"`
	}
	if err := httpx.Decode(r, &req); err != nil {
		httpx.BadRequest(w, "invalid request body")
		return
	}
	if req.AudioRole == "" {
		req.AudioRole = "primary"
	}
	if req.AudioRole != "primary" && req.AudioRole != "audio_alt" {
		httpx.BadRequest(w, "audioRole must be primary or audio_alt")
		return
	}
	if err := h.store.SetMediaFileAudio(r.Context(), id, req.AudioLang, req.AudioRole); err != nil {
		httpx.StoreErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}

// DeleteMediaFile removes a media file: its source on disk, its HLS/frame caches
// and subtitle files, then the row (which cascades the subtitle and transcode
// variant rows). Disk removal is best-effort - the hourly cleanup sweeps any
// leftovers - so a missing file never blocks the delete.
func (h *AdminLibraries) DeleteMediaFile(w http.ResponseWriter, r *http.Request) {
	id := httpx.UUID(r, "id")
	if id == "" {
		httpx.NotFound(w)
		return
	}
	mf, err := h.store.MediaFileByID(r.Context(), id)
	if err != nil {
		httpx.StoreErr(w, err)
		return
	}
	if mf.SourceDeletedAt == nil {
		if lib, lerr := h.store.LibraryByID(r.Context(), mf.LibraryID); lerr == nil {
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
	if err := h.store.DeleteMediaFile(r.Context(), id); err != nil {
		httpx.StoreErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}
