package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"

	"couchverse/internal/auth"
	"couchverse/internal/feature/jobs"
	"couchverse/internal/httpx"
	"couchverse/internal/media"
	"couchverse/internal/settings"
	"couchverse/internal/store"
	"couchverse/internal/transcode"
)

type Stream struct {
	store    *store.Store
	settings *settings.Store
	jobs     *jobs.Store
	dataDir  string
	sessions *transcode.SessionManager
	ffmpeg   string
}

func NewStream(st *store.Store, set *settings.Store, jb *jobs.Store, dataDir string, sessions *transcode.SessionManager, ffmpegPath string) *Stream {
	return &Stream{store: st, settings: set, jobs: jb, dataDir: dataDir, sessions: sessions, ffmpeg: ffmpegPath}
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
	if mf.SourceDeletedAt != nil {
		httpx.Error(w, http.StatusNotFound, "source_deleted", "the original file was removed after transcoding")
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
	Mode           string                `json:"mode"` // direct | unsupported (hls/jit arrive with transcoding)
	MediaFileID    string                `json:"mediaFileId"`
	StreamURL      string                `json:"streamUrl,omitempty"`
	Duration       float64               `json:"durationSeconds"`
	ResumePosition int                   `json:"resumePosition"`
	Display        playbackDisplay       `json:"display"`
	NextEpisode    *store.EpisodeRef     `json:"nextEpisode"`
	Subtitles      []subtitleTrack       `json:"subtitles"`
	Episodes       []store.SeriesEpisode `json:"episodes,omitempty"`
	CurrentEpisode string                `json:"currentEpisodeId,omitempty"`
	HLSURL         string                `json:"hlsUrl,omitempty"`
	Variants       []qualityVariant      `json:"variants,omitempty"`
	JobProgress    int                   `json:"jobProgress,omitempty"`
}

type qualityVariant struct {
	Name   string `json:"name"`
	Height int    `json:"height"`
}

type subtitleTrack struct {
	ID     string `json:"id"`
	Lang   string `json:"lang"`
	Label  string `json:"label"`
	Forced bool   `json:"forced"`
	URL    string `json:"url"`
}

type playbackDisplay struct {
	Title     string `json:"title"`
	Subtitle  string `json:"subtitle"`
	TitleID   string `json:"titleId"`
	TitleSlug string `json:"titleSlug"`
}

// Playback resolves what to play for a movie title or an episode.
func (h *Stream) Playback(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r.Context())
	kind := chi.URLParam(r, "kind")
	id := httpx.UUID(r, "id")
	if id == "" {
		httpx.NotFound(w)
		return
	}

	var (
		mf   *store.MediaFile
		err  error
		info playbackInfo
	)

	switch kind {
	case "movie":
		title, terr := h.store.TitleByID(r.Context(), id)
		if terr != nil {
			httpx.StoreErr(w, terr)
			return
		}
		mf, err = h.store.PrimaryMediaFileForTitle(r.Context(), id)
		if err != nil {
			httpx.Error(w, http.StatusNotFound, "no_media", "this title has no media file yet")
			return
		}
		info.Display = playbackDisplay{Title: title.Name, TitleID: title.ID, TitleSlug: title.Slug}
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
			Title:     ref.TitleName,
			Subtitle:  formatEpisodeSubtitle(ref),
			TitleID:   ref.TitleID,
			TitleSlug: ref.TitleSlug,
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
		if eps, eerr := h.store.PlayableEpisodes(r.Context(), ref.TitleID); eerr == nil {
			info.Episodes = eps
			info.CurrentEpisode = id
		}

	default:
		httpx.BadRequest(w, "kind must be movie or episode")
		return
	}

	info.MediaFileID = mf.ID
	info.Duration = mf.DurationSeconds

	subs, err := h.store.SubtitlesForMediaFile(r.Context(), mf.ID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	info.Subtitles = []subtitleTrack{}
	for _, sub := range subs {
		info.Subtitles = append(info.Subtitles, subtitleTrack{
			ID:     sub.ID,
			Lang:   sub.Lang,
			Label:  sub.Label,
			Forced: sub.Forced,
			URL:    "/api/v1/subtitles/" + sub.ID + ".vtt",
		})
	}

	// ready transcode variants power the player's quality menu and are offered
	// even when the source direct-plays, so users can pick a specific rendition
	variants, verr := h.store.VariantsForMediaFile(r.Context(), mf.ID)
	if verr != nil {
		httpx.Internal(w, verr)
		return
	}
	ready, pending := false, false
	for _, v := range variants {
		switch v.Status {
		case "ready":
			ready = true
			if v.Name != "source" { // the "source" remux isn't a distinct quality
				height := v.Height
				if v.Mode == "copy" {
					height = mf.Height
				}
				info.Variants = append(info.Variants, qualityVariant{Name: v.Name, Height: height})
			}
		case "queued", "processing":
			pending = true
		}
	}
	hlsURL := "/api/v1/stream/" + mf.ID + "/hls/master.m3u8"
	if ready {
		info.HLSURL = hlsURL
	}

	caps := strings.Split(r.URL.Query().Get("caps"), ",")
	switch {
	// a cleaned-up source can't be served directly no matter what the caps say
	case mf.SourceDeletedAt == nil &&
		(mf.DirectPlay || media.DirectPlayWithCaps(mf.Container, mf.VideoCodec, mf.AudioCodec, caps)):
		info.Mode = "direct"
		info.StreamURL = "/api/v1/stream/" + mf.ID

	case ready:
		info.Mode = "hls"
		info.StreamURL = hlsURL
	case pending:
		info.Mode = "preparing"
		if progress, perr := h.jobs.TranscodeProgress(r.Context(), mf.ID); perr == nil {
			info.JobProgress = progress
		}
	case mf.SourceDeletedAt == nil && h.jitAllowed(r.Context()):
		// the client opens a JIT session via POST /stream/{id}/sessions
		info.Mode = "jit"
	default:
		info.Mode = "unsupported"
	}

	httpx.JSON(w, http.StatusOK, info)
}

// HLSMaster generates the master playlist from ready variants.
func (h *Stream) HLSMaster(w http.ResponseWriter, r *http.Request) {
	mediaFileID := httpx.UUID(r, "id")
	if mediaFileID == "" {
		httpx.NotFound(w)
		return
	}
	mf, err := h.store.MediaFileByID(r.Context(), mediaFileID)
	if err != nil {
		httpx.StoreErr(w, err)
		return
	}
	variants, err := h.store.VariantsForMediaFile(r.Context(), mediaFileID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}

	var b strings.Builder
	b.WriteString("#EXTM3U\n#EXT-X-VERSION:3\n")
	count := 0
	for _, v := range variants {
		if v.Status != "ready" {
			continue
		}
		bandwidth := v.VideoBitrate + v.AudioBitrate
		if bandwidth <= 0 {
			bandwidth = mf.Bitrate
		}
		width := v.Width
		height := v.Height
		if v.Mode == "copy" {
			width, height = mf.Width, mf.Height
		}
		if width == 0 && height > 0 && mf.Height > 0 {
			width = mf.Width * height / mf.Height
		}
		fmt.Fprintf(&b, "#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%dx%d,NAME=\"%s\"\n%s/index.m3u8\n",
			bandwidth, width, height, v.Name, v.Name)
		count++
	}
	if count == 0 {
		httpx.NotFound(w)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Cache-Control", "no-cache")
	io.WriteString(w, b.String())
}

// jitAllowed: explicit setting wins; auto enables JIT when a hardware
// encoder was detected (software JIT is usually too slow for live seeking).
func (h *Stream) jitAllowed(ctx context.Context) bool {
	settings := transcode.LoadSettings(ctx, h.settings)
	if settings.JITEnabled != nil {
		return *settings.JITEnabled
	}
	return len(transcode.DetectEncoders(h.ffmpeg)) > 0
}

// CreateSession opens a JIT transcode session.
func (h *Stream) CreateSession(w http.ResponseWriter, r *http.Request) {
	if !h.jitAllowed(r.Context()) {
		httpx.Error(w, http.StatusPreconditionFailed, "jit_disabled", "instant play is disabled")
		return
	}
	var req struct {
		StartAt float64 `json:"startAt"`
	}
	if err := httpx.Decode(r, &req); err != nil {
		httpx.BadRequest(w, "invalid request body")
		return
	}
	mediaFileID := httpx.UUID(r, "id")
	if mediaFileID == "" {
		httpx.NotFound(w)
		return
	}
	if mf, merr := h.store.MediaFileByID(r.Context(), mediaFileID); merr == nil && mf.SourceDeletedAt != nil {
		httpx.Error(w, http.StatusNotFound, "source_deleted", "the original file was removed after transcoding")
		return
	}
	session, err := h.sessions.Create(r.Context(), context.Background(), mediaFileID, max(0, req.StartAt))
	if err != nil {
		httpx.Error(w, http.StatusServiceUnavailable, "session_failed", err.Error())
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]string{
		"sessionId":   session.ID,
		"playlistUrl": "/api/v1/stream/sessions/" + session.ID + "/index.m3u8",
	})
}

func (h *Stream) SessionKeepalive(w http.ResponseWriter, r *http.Request) {
	if !h.sessions.Touch(chi.URLParam(r, "sid")) {
		httpx.NotFound(w)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}

var sessionIDRe = regexp.MustCompile(`^[a-f0-9]{24}$`)

func (h *Stream) SessionFile(w http.ResponseWriter, r *http.Request) {
	sid := chi.URLParam(r, "sid")
	file := chi.URLParam(r, "file")
	if !sessionIDRe.MatchString(sid) {
		httpx.BadRequest(w, "invalid session id")
		return
	}
	session := h.sessions.Get(sid)
	if session == nil {
		httpx.NotFound(w)
		return
	}

	if file == "index.m3u8" {
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Header().Set("Cache-Control", "no-cache")
		io.WriteString(w, session.Playlist())
		return
	}

	path, err := h.sessions.SegmentPath(r.Context(), session, file)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "segment_unavailable", err.Error())
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=60")
	http.ServeFile(w, r, path)
}

var hlsFileRe = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

// HLSFile serves variant playlists and segments from the HLS cache.
func (h *Stream) HLSFile(w http.ResponseWriter, r *http.Request) {
	mediaFileID := httpx.UUID(r, "id")
	if mediaFileID == "" {
		httpx.NotFound(w)
		return
	}
	variant := chi.URLParam(r, "variant")
	file := chi.URLParam(r, "file")
	if !hlsFileRe.MatchString(variant) || !hlsFileRe.MatchString(file) {
		httpx.BadRequest(w, "invalid path")
		return
	}
	path := filepath.Join(h.dataDir, "cache", "hls", mediaFileID, variant, file)
	if strings.HasSuffix(file, ".m3u8") {
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	}
	w.Header().Set("Cache-Control", "private, max-age=3600")
	http.ServeFile(w, r, path)
}

func formatEpisodeSubtitle(ref *store.EpisodeRef) string {
	s := fmt.Sprintf("S%d E%d", ref.SeasonNumber, ref.EpisodeNumber)
	if ref.Name != "" {
		s += " · " + ref.Name
	}
	return s
}
