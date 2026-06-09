package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/go-chi/chi/v5"

	"couchverse/internal/auth"
	"couchverse/internal/httpx"
	"couchverse/internal/store"
)

type Stream struct {
	store *store.Store
}

func NewStream(st *store.Store) *Stream {
	return &Stream{store: st}
}

var contentTypes = map[string]string{
	"mp4":  "video/mp4",
	"m4v":  "video/mp4",
	"webm": "video/webm",
	"mkv":  "video/x-matroska",
	"mp3":  "audio/mpeg",
	"flac": "audio/flac",
	"m4a":  "audio/mp4",
	"ogg":  "audio/ogg",
	"opus": "audio/ogg",
	"wav":  "audio/wav",
}

// Serve streams a media file with HTTP range support (direct play).
func (h *Stream) Serve(w http.ResponseWriter, r *http.Request) {
	mf, err := h.store.MediaFileByID(r.Context(), httpx.ID(r, "id"))
	if err != nil {
		respondStoreErr(w, err)
		return
	}
	lib, err := h.store.LibraryByID(r.Context(), mf.LibraryID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}

	f, err := os.Open(filepath.Join(lib.Path, mf.Path))
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "file_missing", "media file is missing on disk")
		return
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		httpx.Internal(w, err)
		return
	}

	if ct, ok := contentTypes[mf.Container]; ok {
		w.Header().Set("Content-Type", ct)
	}
	http.ServeContent(w, r, filepath.Base(mf.Path), info.ModTime(), f)
}

type playbackInfo struct {
	Mode           string            `json:"mode"` // direct | unsupported (hls/jit arrive with transcoding)
	MediaFileID    int64             `json:"mediaFileId"`
	StreamURL      string            `json:"streamUrl,omitempty"`
	Duration       float64           `json:"durationSeconds"`
	ResumePosition int               `json:"resumePosition"`
	Display        playbackDisplay   `json:"display"`
	NextEpisode    *store.EpisodeRef `json:"nextEpisode"`
}

type playbackDisplay struct {
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	TitleID  int64  `json:"titleId"`
}

// Playback resolves what to play for a movie title or an episode.
func (h *Stream) Playback(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r.Context())
	kind := chi.URLParam(r, "kind")
	id := httpx.ID(r, "id")

	var (
		mf   *store.MediaFile
		err  error
		info playbackInfo
	)

	switch kind {
	case "movie":
		title, terr := h.store.TitleByID(r.Context(), id)
		if terr != nil {
			respondStoreErr(w, terr)
			return
		}
		mf, err = h.store.PrimaryMediaFileForTitle(r.Context(), id)
		if err != nil {
			httpx.Error(w, http.StatusNotFound, "no_media", "this title has no media file yet")
			return
		}
		info.Display = playbackDisplay{Title: title.Name, TitleID: title.ID}
		pos, _, perr := h.store.ProgressFor(r.Context(), user.ID, &id, nil)
		if perr != nil {
			httpx.Internal(w, perr)
			return
		}
		info.ResumePosition = pos

	case "episode":
		ref, rerr := h.store.EpisodeRef(r.Context(), id)
		if rerr != nil {
			httpx.NotFound(w)
			return
		}
		mf, err = h.store.PrimaryMediaFileForEpisode(r.Context(), id)
		if err != nil {
			httpx.Error(w, http.StatusNotFound, "no_media", "this episode has no media file yet")
			return
		}
		info.Display = playbackDisplay{
			Title:    ref.TitleName,
			Subtitle: formatEpisodeSubtitle(ref),
			TitleID:  ref.TitleID,
		}
		pos, _, perr := h.store.ProgressFor(r.Context(), user.ID, nil, &id)
		if perr != nil {
			httpx.Internal(w, perr)
			return
		}
		info.ResumePosition = pos
		if next, nerr := h.store.NextEpisode(r.Context(), id); nerr == nil {
			info.NextEpisode = next
		}

	default:
		httpx.BadRequest(w, "kind must be movie or episode")
		return
	}

	info.MediaFileID = mf.ID
	info.Duration = mf.DurationSeconds
	if mf.DirectPlay {
		info.Mode = "direct"
		info.StreamURL = "/api/v1/stream/" + strconv.FormatInt(mf.ID, 10)
	} else {
		// transcoding pipeline lands in a later milestone
		info.Mode = "unsupported"
	}

	httpx.JSON(w, http.StatusOK, info)
}

func formatEpisodeSubtitle(ref *store.EpisodeRef) string {
	s := fmt.Sprintf("S%d E%d", ref.SeasonNumber, ref.EpisodeNumber)
	if ref.Name != "" {
		s += " · " + ref.Name
	}
	return s
}
