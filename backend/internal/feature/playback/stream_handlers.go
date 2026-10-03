package playback

import (
	"cmp"
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
	"github.com/google/uuid"

	"couchverse/internal/feature/auth"
	"couchverse/internal/feature/catalog"
	"couchverse/internal/feature/jobs"
	"couchverse/internal/feature/library"
	"couchverse/internal/feature/subtitles"
	"couchverse/internal/grant"
	"couchverse/internal/hls"
	"couchverse/internal/httpx"
	"couchverse/internal/media"
	"couchverse/internal/media/mp4"
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
	grants   *grant.Signer
	vtt      vttCache
}

func NewStream(subs *subtitles.Store, cat *catalog.Store, lib *library.Store, set *settings.Store, jb *jobs.Store, dataDir string, sessions *SessionManager, ffmpegPath string, grants *grant.Signer) *Stream {
	return &Stream{subs: subs, catalog: cat, library: lib, settings: set, jobs: jb, dataDir: dataDir, sessions: sessions, ffmpeg: ffmpegPath, grants: grants}
}

// MediaPath is the URL of a route under a media grant: /api/v1/media/{grant}/<rest>.
func MediaPath(g, rest string) string {
	return "/api/v1/media/" + g + "/" + rest
}

// grantedFile is the media file the request's grant names (checked by the media
// group's middleware).
func grantedFile(ctx context.Context) string {
	g, _ := grant.From(ctx)
	return g.Resource.String()
}

// Viewer is who a playback payload is built for.
type Viewer struct {
	// UserID is the signed-in viewer, 0 for an anonymous couch guest; it gets
	// the saved resume position.
	UserID int64
	// Couch is the couch participant id of a follower, "" otherwise; it binds
	// the follower's grants to their place on the couch.
	Couch string
}

// mediaGrant signs a grant for one media file to the viewer.
func (h *Stream) mediaGrant(mediaFileID string, v Viewer) string {
	g := grant.Grant{Scope: grant.Media, Subject: v.UserID}
	var err error
	if g.Resource, err = uuid.Parse(mediaFileID); err != nil {
		return ""
	}
	if v.Couch != "" {
		if g.Couch, err = uuid.Parse(v.Couch); err != nil {
			return ""
		}
	}
	return h.grants.Issue(g, grant.MediaTTL)
}

var contentTypes = map[string]string{
	"mp4":  "video/mp4",
	"m4v":  "video/mp4",
	"mov":  "video/quicktime",
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
	mf, err := h.library.MediaFileByID(r.Context(), grantedFile(r.Context()))
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
		// -ss before -i is a fast input seek to the nearest keyframe; HDR
		// stills are tone-mapped like the transcodes
		filter := "scale=240:-2"
		if isHDR(mf.Video) {
			filter += "," + toneMapFilter(DetectFeatures(h.ffmpeg))
		}
		cmd := exec.CommandContext(r.Context(), h.ffmpeg,
			"-hide_banner", "-loglevel", "error", "-y",
			"-ss", strconv.Itoa(t), "-i", filepath.Join(lib.Path, mf.Path),
			"-frames:v", "1", "-vf", filter, "-q:v", "5", cached)
		if out, err := cmd.CombinedOutput(); err != nil {
			os.Remove(cached)
			httpx.Error(w, http.StatusNotFound, "frame_failed", strings.TrimSpace(string(out)))
			return
		}
	}
	w.Header().Set("Cache-Control", "public, max-age=86400")
	httpx.ServeFile(w, r, cached)
}

// Serve streams a media file with HTTP range support (direct play).
func (h *Stream) Serve(w http.ResponseWriter, r *http.Request) {
	mf, err := h.library.MediaFileByID(r.Context(), grantedFile(r.Context()))
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
	Mode           string                  `json:"mode" enum:"direct,hls,preparing,jit,unsupported" doc:"How to play: direct and hls load streamUrl; preparing waits for a running transcode (see jobProgress); jit opens a session with createStreamSession and the plan in jit; unsupported cannot play."`
	Tier           string                  `json:"tier,omitempty" enum:"direct,remux,transcode" doc:"What reaches the client: the source file, the source video remuxed into HLS, or a transcode. Absent while preparing or unsupported."`
	MediaFileID    string                  `json:"mediaFileId"`
	Grant          string                  `json:"grant" doc:"The media grant every URL in this payload carries; it expires, so fetch the payload again on grant_expired."`
	StreamURL      string                  `json:"streamUrl,omitempty" doc:"The source file (mode direct) or the HLS multivariant playlist (mode hls) to load."`
	FrameURL       string                  `json:"frameUrl" doc:"A still for the seek-bar preview; append ?t=<seconds>."`
	Duration       float64                 `json:"durationSeconds"`
	ResumePosition int                     `json:"resumePosition" doc:"Saved position in seconds; 0 for couch followers, who sync to the host."`
	Display        PlaybackDisplay         `json:"display"`
	NextEpisode    *catalog.EpisodeRef     `json:"nextEpisode,omitempty" doc:"The episode after this one; absent for movies and series finales."`
	Subtitles      []PlaybackSubtitleTrack `json:"subtitles"`
	Audio          []PlaybackAudioTrack    `json:"audio,omitempty" doc:"Selectable audio languages; absent when there is only one."`
	Episodes       []catalog.SeriesEpisode `json:"episodes,omitempty" doc:"The series' playable episodes, for the in-player switcher."`
	CurrentEpisode string                  `json:"currentEpisodeId,omitempty"`
	OriginalURL    string                  `json:"originalUrl,omitempty" doc:"The \"Original\" quality: the source file (tier direct) or the multivariant playlist of the copied source video (tier remux). It stays outside the adaptive ladder."`
	HLSURL         string                  `json:"hlsUrl,omitempty" doc:"Multivariant playlist of the transcoded ladder when it is ready and suits the client, offered even when another tier plays."`
	Variants       []QualityVariant        `json:"variants,omitempty" doc:"Ladder renditions in hlsUrl, for the quality menu."`
	JIT            *JITPlan                `json:"jit,omitempty" doc:"The instant-play session to open (mode jit)."`
	JobProgress    int                     `json:"jobProgress,omitempty" doc:"Transcode progress in percent while mode is preparing."`
	// series opt-in for shuffle playback (drives the player's shuffle toggle)
	AllowRandomPlayback bool `json:"allowRandomPlayback"`
}

// QualityVariant is one ready rendition in the player's quality menu.
type QualityVariant struct {
	Name   string `json:"name"`
	Height int    `json:"height"`
}

type PlaybackSubtitleTrack struct {
	ID     string `json:"id"`
	Lang   string `json:"lang"`
	Label  string `json:"label"`
	Forced bool   `json:"forced"`
	URL    string `json:"url" doc:"The track as WebVTT."`
}

// PlaybackAudioTrack is one selectable audio language. Source "file" (model B)
// is a separate-language media file the player swaps to; "embedded" (model A)
// is an in-stream HLS audio rendition.
type PlaybackAudioTrack struct {
	ID        string `json:"id" doc:"The media file id of a file track; embedded:<stream index> for an embedded one."`
	Lang      string `json:"lang" doc:"Language code (en, cs), as in the HLS audio renditions; und when unknown."`
	Label     string `json:"label"`
	Default   bool   `json:"default"`
	Source    string `json:"source" enum:"file,embedded"`
	StreamURL string `json:"streamUrl,omitempty" doc:"Direct stream of a file track that direct-plays."`
	HLSURL    string `json:"hlsUrl,omitempty" doc:"HLS multivariant playlist of a file track that does not direct-play."`
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
	if n, ok := audioLangNames[bcp47(lang)]; ok {
		return n
	}
	return strings.ToUpper(lang)
}

// PlaybackDisplay is what the player shows about the title being played.
type PlaybackDisplay struct {
	Title          string  `json:"title"`
	Subtitle       string  `json:"subtitle" doc:"The episode line (season, episode and name); empty for movies."`
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

type playbackInput struct {
	Kind string   `path:"kind" enum:"movie,episode"`
	ID   string   `path:"id" format:"uuid" doc:"The movie's title id or the episode id."`
	Caps []string `query:"caps" doc:"Video codecs the client decodes beyond the h264/vp9/av1 baseline (e.g. hevc). Superseded by resolvePlayback, which takes a full device profile."`
}

type resolvePlaybackInput struct {
	Kind string `path:"kind" enum:"movie,episode"`
	ID   string `path:"id" format:"uuid" doc:"The movie's title id or the episode id."`
	Body DeviceProfile
}

type playbackOutput struct{ Body *PlaybackInfo }

// Playback resolves what to play for the browser baseline plus ?caps.
func (h *Stream) Playback(ctx context.Context, in *playbackInput) (*playbackOutput, error) {
	return h.resolve(ctx, in.Kind, in.ID, LegacyProfile(in.Caps))
}

// ResolvePlayback resolves what to play for the device profile in the body.
func (h *Stream) ResolvePlayback(ctx context.Context, in *resolvePlaybackInput) (*playbackOutput, error) {
	return h.resolve(ctx, in.Kind, in.ID, in.Body)
}

func (h *Stream) resolve(ctx context.Context, kind, id string, profile DeviceProfile) (*playbackOutput, error) {
	info, err := h.BuildPlayback(ctx, kind, id, Viewer{UserID: auth.UserFrom(ctx).ID}, profile)
	if errors.Is(err, errNoMedia) {
		return nil, httpx.Fail(http.StatusNotFound, "no_media", "this title has no media file yet")
	}
	if err != nil {
		return nil, err
	}
	return &playbackOutput{Body: info}, nil
}

// BuildPlayback assembles the player payload for a movie title or an episode,
// with media grants issued to the viewer and the tier decided for the device
// profile. A viewer outside a couch gets their saved resume position;
// followers sync to the host instead. It reads no auth and writes no response,
// so the couch feature reuses it to build a follower's payload.
func (h *Stream) BuildPlayback(ctx context.Context, kind, id string, viewer Viewer, profile DeviceProfile) (*PlaybackInfo, error) {
	// resume positions are personal; a couch plays wherever the host is
	userID := viewer.UserID
	if viewer.Couch != "" {
		userID = 0
	}
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
		info.Display = PlaybackDisplay{Title: title.Name, TitleID: title.ID, TitleSlug: title.Slug}
		info.AllowRandomPlayback = title.AllowRandomPlayback
		if userID != 0 {
			pos, _, perr := h.catalog.ProgressFor(ctx, userID, &id, nil)
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
		info.Display = PlaybackDisplay{
			Title:     ref.TitleName,
			Subtitle:  formatEpisodeSubtitle(ref),
			TitleID:   ref.TitleID,
			TitleSlug: ref.TitleSlug,
		}
		// the shuffle flag lives on the title; only the episode ref is loaded above
		if t, terr := h.catalog.TitleByID(ctx, ref.TitleID); terr == nil {
			info.AllowRandomPlayback = t.AllowRandomPlayback
		}
		if userID != 0 {
			pos, _, perr := h.catalog.ProgressFor(ctx, userID, nil, &id)
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
	g := h.mediaGrant(mf.ID, viewer)
	info.Grant = g
	info.FrameURL = MediaPath(g, "frame")

	subs, err := h.subs.SubtitlesForMediaFile(ctx, mf.ID)
	if err != nil {
		return nil, err
	}
	info.Subtitles = []PlaybackSubtitleTrack{}
	for _, sub := range subs {
		info.Subtitles = append(info.Subtitles, PlaybackSubtitleTrack{
			ID:     sub.ID,
			Lang:   sub.Lang,
			Label:  sub.Label,
			Forced: sub.Forced,
			URL:    MediaPath(g, "subtitles/"+sub.ID+".vtt"),
		})
	}

	tracks, err := h.library.AudioStreamsForFile(ctx, mf.ID)
	if err != nil {
		return nil, err
	}
	variants, err := h.library.VariantsForMediaFile(ctx, mf.ID)
	if err != nil {
		return nil, err
	}
	prep := preparedState(variants)
	decision := Decide(profile, sourceFacts(mf, tracks, len(subs)), prep, h.server(ctx))
	h.apply(ctx, &info, decision, mf, g, variants)

	// alternate-audio siblings (model B): a language switch in the player that
	// swaps the whole file. Only populated when there is more than one file.
	if siblings, serr := h.catalog.AudioSiblings(ctx, mf.TitleID, mf.EpisodeID, mf.ID); serr == nil && len(siblings) > 0 {
		info.Audio = append(info.Audio, h.fileTrack(ctx, mf, info.StreamURL, info.Mode, true))
		for i := range siblings {
			sg := h.mediaGrant(siblings[i].ID, viewer)
			si := h.siblingPlayback(ctx, &siblings[i], profile, sg)
			info.Audio = append(info.Audio, h.fileTrack(ctx, &siblings[i], si.StreamURL, si.Mode, false))
		}
	} else if len(tracks) >= 2 {
		// embedded tracks (model A) switch inside the HLS audio group
		info.Audio = embeddedTracks(tracks)
	}
	return &info, nil
}

// apply turns a decision into the payload's mode and URLs.
func (h *Stream) apply(ctx context.Context, info *PlaybackInfo, d Decision, mf *media.MediaFile, g string, variants []library.TranscodeVariant) {
	info.Mode, info.Tier, info.JIT = d.Mode, d.Tier, d.JIT
	switch d.Mode {
	case "direct":
		info.StreamURL = MediaPath(g, "stream")
		info.OriginalURL = info.StreamURL
	case "hls":
		info.StreamURL = masterURL(g, d.Master)
		if d.Master.Video == "legacy" && legacyMultiAudio(variants) {
			info.StreamURL = MediaPath(g, "hls/multiaudio/master.m3u8")
		}
		if d.Tier == TierRemux {
			info.OriginalURL = info.StreamURL
		}
	case "preparing":
		if progress, err := h.jobs.TranscodeProgress(ctx, mf.ID); err == nil {
			info.JobProgress = progress
		}
	}
	if d.Ladder {
		info.HLSURL = masterURL(g, Master{Video: "ladder", Surround: d.Master.Surround})
		for _, v := range variants {
			if _, rung := media.Renditions[v.Name]; rung && v.Status == "ready" && v.Format == "fmp4" {
				info.Variants = append(info.Variants, QualityVariant{Name: v.Name, Height: v.Height})
			}
		}
	}
}

// siblingPlayback decides how a model-B sibling file plays for the same profile.
func (h *Stream) siblingPlayback(ctx context.Context, mf *media.MediaFile, profile DeviceProfile, g string) PlaybackInfo {
	var info PlaybackInfo
	tracks, _ := h.library.AudioStreamsForFile(ctx, mf.ID)
	variants, _ := h.library.VariantsForMediaFile(ctx, mf.ID)
	d := Decide(profile, sourceFacts(mf, tracks, 0), preparedState(variants), Server{})
	h.apply(ctx, &info, d, mf, g, variants)
	return info
}

func (h *Stream) fileTrack(_ context.Context, mf *media.MediaFile, url, mode string, isDefault bool) PlaybackAudioTrack {
	t := PlaybackAudioTrack{ID: mf.ID, Lang: mf.AudioLang, Label: audioLabel(mf.AudioLang), Default: isDefault, Source: "file"}
	switch mode {
	case "direct":
		t.StreamURL = url
	case "hls":
		t.HLSURL = url
	}
	return t
}

func embeddedTracks(tracks []media.AudioStream) []PlaybackAudioTrack {
	out := []PlaybackAudioTrack{}
	hasDefault := false
	for _, a := range tracks {
		label := a.Title
		if label == "" || label == a.Lang {
			label = audioLabel(a.Lang)
		}
		out = append(out, PlaybackAudioTrack{
			ID:      fmt.Sprintf("embedded:%d", a.Index),
			Lang:    bcp47(a.Lang),
			Label:   label,
			Default: a.Default && !hasDefault,
			Source:  "embedded",
		})
		hasDefault = hasDefault || a.Default
	}
	if !hasDefault && len(out) > 0 {
		out[0].Default = true
	}
	return out
}

// sourceFacts describes a media file for the decision. Files the prober has
// not read again since Playback v2 only know their video range.
func sourceFacts(mf *media.MediaFile, tracks []media.AudioStream, subtitles int) Source {
	v := mf.Video
	if mf.ProbeVersion < 2 {
		switch mf.VideoRange {
		case "hdr10":
			v.HDR = media.HDR10
		case "hlg":
			v.HDR = media.HLG
		}
	}
	if len(tracks) == 0 && mf.AudioCodec != "" {
		tracks = []media.AudioStream{{Index: -1, Codec: mf.AudioCodec, Channels: mf.Channels, Default: true}}
	}
	return Source{
		Container: mf.Container, VideoCodec: mf.VideoCodec, Video: v,
		Width: mf.Width, Height: mf.Height, Bitrate: mf.Bitrate,
		Audio: tracks, Subtitles: subtitles, Available: mf.SourceDeletedAt == nil,
	}
}

// preparedState summarizes the variant rows for the decision.
func preparedState(variants []library.TranscodeVariant) Prepared {
	var p Prepared
	better := func(cur *PackageState, status string) {
		switch {
		case status == "ready":
			*cur = PackageReady
		case (status == "queued" || status == "processing") && *cur == PackageMissing:
			*cur = PackagePending
		}
	}
	for _, v := range variants {
		if v.Format == "ts" {
			if v.Name == "multiaudio" {
				p.LegacyMultiAudio = p.LegacyMultiAudio || v.Status == "ready"
			}
			better(&p.Legacy, v.Status)
			continue
		}
		switch v.Name {
		case media.VariantSource:
			better(&p.Original, v.Status)
		case media.VariantAudio:
			better(&p.Audio, v.Status)
		case media.VariantTrickplay:
		default:
			better(&p.Ladder, v.Status)
		}
	}
	return p
}

func legacyMultiAudio(variants []library.TranscodeVariant) bool {
	return preparedState(variants).LegacyMultiAudio
}

// server reports what this server can deliver right now.
func (h *Stream) server(ctx context.Context) Server {
	return Server{JIT: h.jitAllowed(ctx), DolbyVisionCopy: DetectFeatures(h.ffmpeg).DolbyVision}
}

// HLSMaster writes the multivariant playlist a payload points at: the copied
// source (?video=original) or the transcoded ladder (?video=ladder) with the
// surround codecs the client takes, or the pre-v2 MPEG-TS variants (?video=legacy).
func (h *Stream) HLSMaster(w http.ResponseWriter, r *http.Request) {
	mediaFileID := grantedFile(r.Context())
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
	q := r.URL.Query()
	video := q.Get("video")
	var body string
	switch video {
	case "legacy":
		body = legacyMaster(mf, variants)
		if !strings.Contains(body, "#EXT-X-STREAM-INF") {
			httpx.NotFound(w)
			return
		}
	case "", "ladder", "original":
		subs, err := h.subs.SubtitlesForMediaFile(r.Context(), mediaFileID)
		if err != nil {
			httpx.Internal(w, err)
			return
		}
		m := Master{Video: video}
		if m.Video == "" {
			m.Video = "ladder"
		}
		if s := q.Get("surround"); s != "" {
			m.Surround = strings.Split(s, ",")
		}
		pl, err := buildMaster(h.renditions(mediaFileID, variants), subs, m)
		if err != nil {
			httpx.Error(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		body = pl.String()
	default:
		httpx.BadRequest(w, "video must be original, ladder or legacy")
		return
	}
	w.Header().Set("Content-Type", playlistType)
	w.Header().Set("Cache-Control", "no-cache")
	io.WriteString(w, body)
}

var subtitleFileRe = regexp.MustCompile(`^(index\.m3u8|\d{1,6}\.vtt)$`)

// HLSSubtitleFile serves a sidecar subtitle track as an HLS rendition: a media
// playlist on the video's segment grid and WebVTT segments mapped onto the
// shared timeline.
func (h *Stream) HLSSubtitleFile(w http.ResponseWriter, r *http.Request) {
	id, file := httpx.UUID(r, "id"), chi.URLParam(r, "file")
	if id == "" || !subtitleFileRe.MatchString(file) {
		httpx.NotFound(w)
		return
	}
	sub, err := h.subs.SubtitleByID(r.Context(), id)
	if err != nil {
		httpx.StoreErr(w, err)
		return
	}
	mediaFileID := grantedFile(r.Context())
	if sub.MediaFileID != mediaFileID {
		httpx.NotFound(w)
		return
	}
	mf, err := h.library.MediaFileByID(r.Context(), mediaFileID)
	if err != nil {
		httpx.StoreErr(w, err)
		return
	}
	if file == "index.m3u8" {
		w.Header().Set("Content-Type", playlistType)
		w.Header().Set("Cache-Control", "private, max-age=3600")
		io.WriteString(w, subtitlePlaylist(mf.DurationSeconds).String())
		return
	}
	n, _ := strconv.Atoi(strings.TrimSuffix(file, ".vtt"))
	start := float64(n) * segmentSeconds
	if start >= mf.DurationSeconds {
		httpx.NotFound(w)
		return
	}
	vtt, err := h.vtt.get(r.Context(), filepath.Join(h.dataDir, sub.Path))
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "subtitle_unreadable", err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	io.WriteString(w, vtt.Segment(start, start+segmentSeconds, subtitleMPEGTS))
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

// requireJIT gates opening a JIT session. It runs before the request is
// parsed, so a disabled instant play is reported ahead of any input error.
func (h *Stream) requireJIT(ctx context.Context) error {
	if !h.jitAllowed(ctx) {
		return httpx.Fail(http.StatusPreconditionFailed, "jit_disabled", "instant play is disabled")
	}
	return nil
}

// StreamSessionStart opens an instant-play session; the plan fields come from
// the payload's jit object and default to an H.264 transcode with AAC of the
// default audio track.
type StreamSessionStart struct {
	StartAt     float64 `json:"startAt" required:"false" doc:"Position in seconds to start transcoding from."`
	Video       string  `json:"video,omitempty" required:"false" enum:"copy,transcode" doc:"From jit.video; the session transcodes when it cannot copy."`
	AudioStream *int    `json:"audioStream,omitempty" required:"false" doc:"From jit.audioStream: the source stream index of the audio track."`
	Audio       string  `json:"audio,omitempty" required:"false" enum:"copy,aac,eac3" doc:"From jit.audio."`
}

type createStreamSessionInput struct {
	Body StreamSessionStart
}

// StreamSession is an open JIT transcode. Play PlaylistURL and keep the
// session alive while watching; idle sessions are reaped.
type StreamSession struct {
	SessionID   string `json:"sessionId"`
	PlaylistURL string `json:"playlistUrl" doc:"The session's multivariant playlist (with the subtitle renditions)."`
}

type streamSessionOutput struct{ Body StreamSession }

// CreateSession opens a JIT session for the granted media file, bound to the
// grant's viewer.
func (h *Stream) CreateSession(ctx context.Context, in *createStreamSessionInput) (*streamSessionOutput, error) {
	g, _ := grant.From(ctx)
	mediaFileID := g.Resource.String()
	if mf, merr := h.library.MediaFileByID(ctx, mediaFileID); merr == nil && mf.SourceDeletedAt != nil {
		return nil, httpx.Fail(http.StatusNotFound, "source_deleted", "the original file was removed after transcoding")
	}
	plan := JITPlan{Video: in.Body.Video, AudioStream: -1, Audio: in.Body.Audio}
	if in.Body.AudioStream != nil {
		plan.AudioStream = *in.Body.AudioStream
	}
	owner := sessionOwner{Subject: g.Subject, Couch: g.Couch}
	session, err := h.sessions.Create(ctx, context.Background(), mediaFileID, owner, plan, max(0, in.Body.StartAt))
	if err != nil {
		return nil, httpx.Fail(http.StatusServiceUnavailable, "session_failed", err.Error())
	}
	return &streamSessionOutput{Body: StreamSession{
		SessionID: session.ID,
		// the grant in the path lets relative segment URIs inherit it
		PlaylistURL: MediaPath(h.grants.Sign(g), "jit/"+session.ID+"/master.m3u8"),
	}}, nil
}

type sessionInput struct {
	SID string `path:"sid" pattern:"^[a-f0-9]{24}$"`
}

// grantedSession finds a JIT session opened with the request's grant: the same
// media file and the same viewer, so one viewer never reaches another's.
func (h *Stream) grantedSession(ctx context.Context, sid string) *Session {
	session := h.sessions.Get(sid)
	g, _ := grant.From(ctx)
	if session == nil || session.MediaFileID != g.Resource.String() ||
		session.owner != (sessionOwner{Subject: g.Subject, Couch: g.Couch}) {
		return nil
	}
	return session
}

func (h *Stream) SessionKeepalive(ctx context.Context, in *sessionInput) (*struct{}, error) {
	if h.grantedSession(ctx, in.SID) == nil || !h.sessions.Touch(in.SID) {
		return nil, httpx.NotFoundError()
	}
	return nil, nil
}

// StopSession ends a JIT session as soon as the player leaves, instead of
// holding the transcoder until the idle reaper notices.
func (h *Stream) StopSession(ctx context.Context, in *sessionInput) (*struct{}, error) {
	if h.grantedSession(ctx, in.SID) == nil {
		return nil, httpx.NotFoundError()
	}
	h.sessions.Stop(in.SID)
	return nil, nil
}

var sessionIDRe = regexp.MustCompile(`^[a-f0-9]{24}$`)

func (h *Stream) SessionFile(w http.ResponseWriter, r *http.Request) {
	sid := chi.URLParam(r, "sid")
	file := chi.URLParam(r, "file")
	if !sessionIDRe.MatchString(sid) {
		httpx.BadRequest(w, "invalid session id")
		return
	}
	session := h.grantedSession(r.Context(), sid)
	if session == nil {
		httpx.NotFound(w)
		return
	}

	switch file {
	case "master.m3u8":
		body, err := h.sessionMaster(r.Context(), session)
		if err != nil {
			httpx.Error(w, http.StatusNotFound, "segment_unavailable", err.Error())
			return
		}
		w.Header().Set("Content-Type", playlistType)
		w.Header().Set("Cache-Control", "no-cache")
		io.WriteString(w, body)
		return
	case "index.m3u8":
		w.Header().Set("Content-Type", playlistType)
		w.Header().Set("Cache-Control", "no-cache")
		io.WriteString(w, session.Playlist())
		return
	}

	var (
		path string
		err  error
	)
	if file == "init.mp4" {
		path, err = h.sessions.InitPath(r.Context(), session)
	} else {
		path, err = h.sessions.SegmentPath(r.Context(), session, file)
	}
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "segment_unavailable", err.Error())
		return
	}
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Cache-Control", "private, max-age=60")
	httpx.ServeFile(w, r, path)
}

// sessionMaster describes a JIT session as a multivariant playlist, so players
// get its codecs and the subtitle renditions. The codecs come from the first
// run's init section, which the encoder writes within a second or two.
func (h *Stream) sessionMaster(ctx context.Context, s *Session) (string, error) {
	path, err := h.sessions.InitPath(ctx, s)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	in, err := mp4.ParseInit(data)
	if err != nil {
		return "", err
	}
	v := hls.Variant{ClosedCaptions: "NONE", URI: "index.m3u8", VideoRange: "SDR",
		FrameRate: float64(int(s.mf.Video.FrameRate*1000+0.5)) / 1000}
	bandwidth := s.rend.VideoBitrate
	for _, t := range in.Tracks {
		v.Codecs = append(v.Codecs, t.Codec)
		if t.Handler == "vide" {
			v.Width, v.Height = t.Width, t.Height
		} else if t.Codec == "mp4a.40.2" {
			bandwidth += 160_000
		} else {
			bandwidth += 640_000
		}
	}
	// a peak, with room for the encoder's rate control
	v.Bandwidth, v.AverageBandwidth = bandwidth*12/10, bandwidth

	pl := &hls.Multivariant{Version: 7, IndependentSegments: true}
	subs, err := h.subs.SubtitlesForMediaFile(ctx, s.MediaFileID)
	if err != nil {
		return "", err
	}
	names := uniqueNames{}
	for _, sub := range subs {
		v.Subtitles = "subs"
		pl.Renditions = append(pl.Renditions, hls.Rendition{
			Type: "SUBTITLES", GroupID: "subs", Name: names.take(cmp.Or(sub.Label, audioLabel(sub.Lang))),
			Language: bcp47(sub.Lang), Autoselect: true, Forced: sub.Forced,
			URI: "../../hls/subtitles/" + sub.ID + "/index.m3u8",
		})
	}
	pl.Variants = []hls.Variant{v}
	return pl.String(), nil
}

var hlsFileRe = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

// HLSFile serves variant playlists, init sections and segments from the HLS cache.
func (h *Stream) HLSFile(w http.ResponseWriter, r *http.Request) {
	mediaFileID := grantedFile(r.Context())
	variant := chi.URLParam(r, "variant")
	file := chi.URLParam(r, "file")
	if !hlsFileRe.MatchString(variant) || !hlsFileRe.MatchString(file) || file == renditionFile {
		httpx.BadRequest(w, "invalid path")
		return
	}
	path := filepath.Join(h.dataDir, "cache", "hls", mediaFileID, variant, file)
	if ct := hlsContentType(variant, file); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	w.Header().Set("Cache-Control", "private, max-age=3600")
	httpx.ServeFile(w, r, path)
}

// hlsContentType names HLS files explicitly: Go's extension table lacks .m4s
// and maps .ts to TypeScript on some systems, which strict players reject.
// Audio renditions live in audio-* directories.
func hlsContentType(dir, file string) string {
	switch filepath.Ext(file) {
	case ".m3u8":
		return playlistType
	case ".ts":
		return "video/mp2t"
	case ".m4s", ".mp4":
		if strings.HasPrefix(dir, "audio-") {
			return "audio/mp4"
		}
		return "video/mp4"
	case ".vtt", ".webvtt":
		return "text/vtt"
	case ".aac":
		return "audio/aac"
	}
	return ""
}

func formatEpisodeSubtitle(ref *catalog.EpisodeRef) string {
	s := fmt.Sprintf("S%d E%d", ref.SeasonNumber, ref.EpisodeNumber)
	if ref.Name != "" {
		s += " · " + ref.Name
	}
	return s
}
