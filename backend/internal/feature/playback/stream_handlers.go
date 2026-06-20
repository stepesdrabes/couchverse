package playback

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"couchverse/internal/feature/auth"
	"couchverse/internal/feature/catalog"
	"couchverse/internal/feature/jobs"
	"couchverse/internal/feature/library"
	"couchverse/internal/feature/subtitles"
	"couchverse/internal/httpx"
	"couchverse/internal/media"
	"couchverse/internal/settings"
)

type Stream struct {
	subs     *subtitles.Store
	catalog  *catalog.Store
	library  *library.Store
	settings *settings.Store
	jobs     *jobs.Store
	dataDir  string
	sessions *SessionManager
	ffmpeg   string
}

func NewStream(subs *subtitles.Store, cat *catalog.Store, lib *library.Store, set *settings.Store, jb *jobs.Store, dataDir string, sessions *SessionManager, ffmpegPath string) *Stream {
	return &Stream{subs: subs, catalog: cat, library: lib, settings: set, jobs: jb, dataDir: dataDir, sessions: sessions, ffmpeg: ffmpegPath}
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

// Frame returns a single still from the source video at ?t=SECONDS, scaled
// down for the player's seek-bar preview. Requests are bucketed to a few
// seconds and cached so scrubbing reuses extracted frames.
func (h *Stream) Frame(w http.ResponseWriter, r *http.Request) {
	id := httpx.UUID(r, "id")
	if id == "" {
		httpx.NotFound(w)
		return
	}
	mf, err := h.library.MediaFileByID(r.Context(), id)
	if err != nil {
		httpx.StoreErr(w, err)
		return
	}
	if mf.SourceDeletedAt != nil || mf.VideoCodec == "" {
		httpx.NotFound(w) // no source on disk to grab a frame from
		return
	}

	const bucket = 5 // seconds; coarse enough to bound the cache and reuse hovers
	t := httpx.QueryInt(r, "t", 0)
	if t < 0 {
		t = 0
	}
	if mf.DurationSeconds > 0 && float64(t) > mf.DurationSeconds {
		t = int(mf.DurationSeconds)
	}
	t = (t / bucket) * bucket

	dir := filepath.Join(h.dataDir, "cache", "frames", mf.ID)
	cached := filepath.Join(dir, strconv.Itoa(t)+".jpg")
	if _, err := os.Stat(cached); err != nil {
		lib, lerr := h.library.LibraryByID(r.Context(), mf.LibraryID)
		if lerr != nil {
			httpx.Internal(w, lerr)
			return
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			httpx.Internal(w, err)
			return
		}
		// -ss before -i is a fast input seek to the nearest keyframe
		cmd := exec.CommandContext(r.Context(), h.ffmpeg,
			"-hide_banner", "-loglevel", "error", "-y",
			"-ss", strconv.Itoa(t), "-i", filepath.Join(lib.Path, mf.Path),
			"-frames:v", "1", "-vf", "scale=240:-2", "-q:v", "5", cached)
		if out, err := cmd.CombinedOutput(); err != nil {
			os.Remove(cached)
			httpx.Error(w, http.StatusNotFound, "frame_failed", strings.TrimSpace(string(out)))
			return
		}
	}
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeFile(w, r, cached)
}

// Serve streams a media file with HTTP range support (direct play).
func (h *Stream) Serve(w http.ResponseWriter, r *http.Request) {
	id := httpx.UUID(r, "id")
	if id == "" {
		httpx.NotFound(w)
		return
	}
	mf, err := h.library.MediaFileByID(r.Context(), id)
	if err != nil {
		httpx.StoreErr(w, err)
		return
	}
	if mf.SourceDeletedAt != nil {
		httpx.Error(w, http.StatusNotFound, "source_deleted", "the original file was removed after transcoding")
		return
	}
	lib, err := h.library.LibraryByID(r.Context(), mf.LibraryID)
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

// PlaybackInfo is the player payload. It is built by BuildPlayback and reused
// by the couch feature to assemble a follower's player without an auth context.
type PlaybackInfo struct {
	Mode           string                  `json:"mode"` // direct | unsupported (hls/jit arrive with transcoding)
	MediaFileID    string                  `json:"mediaFileId"`
	StreamURL      string                  `json:"streamUrl,omitempty"`
	Duration       float64                 `json:"durationSeconds"`
	ResumePosition int                     `json:"resumePosition"`
	Display        playbackDisplay         `json:"display"`
	NextEpisode    *catalog.EpisodeRef     `json:"nextEpisode"`
	Subtitles      []subtitleTrack         `json:"subtitles"`
	Audio          []audioTrack            `json:"audio,omitempty"`
	Episodes       []catalog.SeriesEpisode `json:"episodes,omitempty"`
	CurrentEpisode string                  `json:"currentEpisodeId,omitempty"`
	HLSURL         string                  `json:"hlsUrl,omitempty"`
	Variants       []qualityVariant        `json:"variants,omitempty"`
	JobProgress    int                     `json:"jobProgress,omitempty"`
	// series opt-in for shuffle playback (drives the player's shuffle toggle)
	AllowRandomPlayback bool `json:"allowRandomPlayback"`
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

// audioTrack is one selectable audio language. Source "file" (model B) is a
// separate-language media file the player swaps to; "embedded" (model A) is an
// in-stream HLS audio rendition.
type audioTrack struct {
	ID        string `json:"id"`
	Lang      string `json:"lang"`
	Label     string `json:"label"`
	Default   bool   `json:"default"`
	Source    string `json:"source"`
	StreamURL string `json:"streamUrl,omitempty"`
	HLSURL    string `json:"hlsUrl,omitempty"`
}

var audioLangNames = map[string]string{
	"en": "English", "cs": "Čeština", "sk": "Slovenčina", "de": "Deutsch",
	"es": "Español", "fr": "Français", "it": "Italiano", "pl": "Polski",
	"ko": "한국어", "ja": "日本語", "ru": "Русский", "zh": "中文",
}

func audioLabel(lang string) string {
	if lang == "" || lang == "und" {
		return "Original"
	}
	if n, ok := audioLangNames[lang]; ok {
		return n
	}
	return strings.ToUpper(lang)
}

// audioTrackFor builds a model-B track from a media file, pointing at its direct
// stream when it direct-plays, or its HLS master otherwise.
func audioTrackFor(mf *media.MediaFile, isDefault bool) audioTrack {
	t := audioTrack{ID: mf.ID, Lang: mf.AudioLang, Label: audioLabel(mf.AudioLang), Default: isDefault, Source: "file"}
	if mf.SourceDeletedAt == nil && mf.DirectPlay {
		t.StreamURL = "/api/v1/stream/" + mf.ID
	} else {
		t.HLSURL = "/api/v1/stream/" + mf.ID + "/hls/master.m3u8"
	}
	return t
}

type playbackDisplay struct {
	Title          string  `json:"title"`
	Subtitle       string  `json:"subtitle"`
	TitleID        string  `json:"titleId"`
	TitleSlug      string  `json:"titleSlug"`
	BackdropID     *string `json:"backdropId"`
	BackdropVer    int64   `json:"backdropVer,omitempty"`
	BackdropAccent string  `json:"backdropAccent,omitempty"`
}

var (
	errNoMedia = errors.New("playback: title or episode has no media file")
	errBadKind = errors.New("playback: kind must be movie or episode")
)

// Playback resolves what to play for a movie title or an episode.
func (h *Stream) Playback(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r.Context())
	var uid *int64
	if user != nil {
		uid = &user.ID
	}
	caps := strings.Split(r.URL.Query().Get("caps"), ",")
	info, err := h.BuildPlayback(r.Context(), chi.URLParam(r, "kind"), httpx.UUID(r, "id"), uid, caps)
	switch {
	case err == nil:
		httpx.JSON(w, http.StatusOK, info)
	case errors.Is(err, errBadKind):
		httpx.BadRequest(w, "kind must be movie or episode")
	case errors.Is(err, errNoMedia):
		httpx.Error(w, http.StatusNotFound, "no_media", "this title has no media file yet")
	default:
		httpx.StoreErr(w, err)
	}
}

// BuildPlayback assembles the player payload for a movie title or an episode.
// When userID is non-nil the viewer's saved resume position is included; couch
// followers pass nil (they sync to the host, not their own progress). It reads
// no auth and writes no response, so the couch feature reuses it to build a
// follower's payload without the viewer being logged in. caps is the client's
// container/codec capability list (used for the direct-play decision).
func (h *Stream) BuildPlayback(ctx context.Context, kind, id string, userID *int64, caps []string) (*PlaybackInfo, error) {
	if id == "" {
		return nil, httpx.ErrNotFound
	}

	var (
		mf   *media.MediaFile
		err  error
		info PlaybackInfo
	)

	switch kind {
	case "movie":
		title, terr := h.catalog.TitleByID(ctx, id)
		if terr != nil {
			return nil, terr
		}
		mf, err = h.catalog.PrimaryMediaFileForTitle(ctx, id)
		if err != nil {
			return nil, errNoMedia
		}
		info.Display = playbackDisplay{Title: title.Name, TitleID: title.ID, TitleSlug: title.Slug}
		info.AllowRandomPlayback = title.AllowRandomPlayback
		if userID != nil {
			pos, _, perr := h.catalog.ProgressFor(ctx, *userID, &id, nil)
			if perr != nil {
				return nil, perr
			}
			info.ResumePosition = pos
		}

	case "episode":
		ref, rerr := h.catalog.EpisodeRef(ctx, id)
		if rerr != nil {
			return nil, httpx.ErrNotFound
		}
		mf, err = h.catalog.PrimaryMediaFileForEpisode(ctx, id)
		if err != nil {
			return nil, errNoMedia
		}
		info.Display = playbackDisplay{
			Title:     ref.TitleName,
			Subtitle:  formatEpisodeSubtitle(ref),
			TitleID:   ref.TitleID,
			TitleSlug: ref.TitleSlug,
		}
		// the shuffle flag lives on the title; only the episode ref is loaded above
		if t, terr := h.catalog.TitleByID(ctx, ref.TitleID); terr == nil {
			info.AllowRandomPlayback = t.AllowRandomPlayback
		}
		if userID != nil {
			pos, _, perr := h.catalog.ProgressFor(ctx, *userID, nil, &id)
			if perr != nil {
				return nil, perr
			}
			info.ResumePosition = pos
		}
		if next, nerr := h.catalog.NextEpisode(ctx, id); nerr == nil {
			info.NextEpisode = next
		}
		if eps, eerr := h.catalog.PlayableEpisodes(ctx, ref.TitleID); eerr == nil {
			info.Episodes = eps
			info.CurrentEpisode = id
		}

	default:
		return nil, errBadKind
	}

	// banner accent for the player (best effort)
	if bid, bver, accent, berr := h.catalog.TitleBackdrop(ctx, info.Display.TitleID); berr == nil {
		info.Display.BackdropID = bid
		info.Display.BackdropVer = bver
		info.Display.BackdropAccent = accent
	}

	info.MediaFileID = mf.ID
	info.Duration = mf.DurationSeconds

	subs, err := h.subs.SubtitlesForMediaFile(ctx, mf.ID)
	if err != nil {
		return nil, err
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

	// alternate-audio siblings (model B): a language switch in the player. Only
	// populated when the title/episode has more than one audio file.
	if siblings, serr := h.catalog.AudioSiblings(ctx, mf.TitleID, mf.EpisodeID, mf.ID); serr == nil && len(siblings) > 0 {
		info.Audio = append(info.Audio, audioTrackFor(mf, true))
		for i := range siblings {
			info.Audio = append(info.Audio, audioTrackFor(&siblings[i], false))
		}
	}

	// embedded multi-audio (model A): a file with >=2 audio tracks streams via the
	// var_stream_map HLS remux so the player can switch audio language.
	audioStreams, _ := h.library.AudioStreamsForFile(ctx, mf.ID)
	multiAudio := len(audioStreams) >= 2

	// ready transcode variants power the player's quality menu and are offered
	// even when the source direct-plays, so users can pick a specific rendition
	variants, verr := h.library.VariantsForMediaFile(ctx, mf.ID)
	if verr != nil {
		return nil, verr
	}
	ready, pending, multiAudioReady := false, false, false
	for _, v := range variants {
		switch v.Status {
		case "ready":
			ready = true
			switch v.Name {
			case "multiaudio":
				multiAudioReady = true
			case "source": // the "source" remux isn't a distinct quality
			default:
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
	if multiAudio && multiAudioReady {
		hlsURL = "/api/v1/stream/" + mf.ID + "/hls/multiaudio/master.m3u8"
	}
	if ready {
		info.HLSURL = hlsURL
	}

	// embedded audio tracks for the player's language menu (model A)
	if multiAudio && len(info.Audio) == 0 {
		for _, a := range audioStreams {
			label := a.Title
			if label == "" || label == a.Lang {
				label = audioLabel(a.Lang)
			}
			info.Audio = append(info.Audio, audioTrack{
				ID:      fmt.Sprintf("embedded:%d", a.Index),
				Lang:    a.Lang,
				Label:   label,
				Default: a.Default,
				Source:  "embedded",
			})
		}
		hasDef := false
		for _, t := range info.Audio {
			if t.Default {
				hasDef = true
			}
		}
		if !hasDef && len(info.Audio) > 0 {
			info.Audio[0].Default = true
		}
	}

	switch {
	// embedded multi-audio must use the HLS remux so the player can switch audio,
	// even when the source would otherwise direct-play
	case multiAudio && multiAudioReady:
		info.Mode = "hls"
		info.StreamURL = hlsURL

	// a cleaned-up source can't be served directly no matter what the caps say
	case mf.SourceDeletedAt == nil && !multiAudio &&
		(mf.DirectPlay || media.DirectPlayWithCaps(mf.Container, mf.VideoCodec, mf.AudioCodec, caps)):
		info.Mode = "direct"
		info.StreamURL = "/api/v1/stream/" + mf.ID

	case ready:
		info.Mode = "hls"
		info.StreamURL = hlsURL
	case pending:
		info.Mode = "preparing"
		if progress, perr := h.jobs.TranscodeProgress(ctx, mf.ID); perr == nil {
			info.JobProgress = progress
		}
	case mf.SourceDeletedAt == nil && h.jitAllowed(ctx):
		// the client opens a JIT session via POST /stream/{id}/sessions
		info.Mode = "jit"
	default:
		info.Mode = "unsupported"
	}

	return &info, nil
}

// HLSMaster generates the master playlist from ready variants.
func (h *Stream) HLSMaster(w http.ResponseWriter, r *http.Request) {
	mediaFileID := httpx.UUID(r, "id")
	if mediaFileID == "" {
		httpx.NotFound(w)
		return
	}
	mf, err := h.library.MediaFileByID(r.Context(), mediaFileID)
	if err != nil {
		httpx.StoreErr(w, err)
		return
	}
	variants, err := h.library.VariantsForMediaFile(r.Context(), mediaFileID)
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
	settings := media.LoadTranscodeSettings(ctx, h.settings)
	if settings.JITEnabled != nil {
		return *settings.JITEnabled
	}
	return len(DetectEncoders(h.ffmpeg)) > 0
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
	if mf, merr := h.library.MediaFileByID(r.Context(), mediaFileID); merr == nil && mf.SourceDeletedAt != nil {
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

func formatEpisodeSubtitle(ref *catalog.EpisodeRef) string {
	s := fmt.Sprintf("S%d E%d", ref.SeasonNumber, ref.EpisodeNumber)
	if ref.Name != "" {
		s += " · " + ref.Name
	}
	return s
}
