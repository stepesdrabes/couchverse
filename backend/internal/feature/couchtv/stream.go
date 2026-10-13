package couchtv

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"couchverse/internal/feature/playback"
	"couchverse/internal/hls"
	"couchverse/internal/httpx"
	"couchverse/internal/media/mp4"
)

// Live is the channel's live stream: the playlists computed from the schedule
// and the files they list, served under a channel grant (/tv/{grant}/...).
type Live struct {
	Channel *Channel
	Slate   *Slate
	media   *DiskMedia
	store   *Store
	dataDir string
	vtt     vttCache
}

func NewLive(st *Store, dataDir, ffmpeg string) *Live {
	slate := NewSlate(dataDir, ffmpeg)
	m := NewDiskMedia(dataDir, slate)
	return &Live{Channel: NewChannel(st, m), Slate: slate, media: m, store: st, dataDir: dataDir}
}

const playlistType = "application/vnd.apple.mpegurl"

func writePlaylist(w http.ResponseWriter, text string) {
	w.Header().Set("Content-Type", playlistType)
	w.Header().Set("Cache-Control", "no-cache")
	io.WriteString(w, text)
}

// failed answers a playlist request that failed: nothing on air, or a player
// that gave up waiting (players abort requests as they seek or switch).
func failed(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, hls.ErrOffAir):
		httpx.Error(w, http.StatusNotFound, "off_air", "nothing is on air")
	case errors.Is(err, context.Canceled):
	default:
		httpx.Internal(w, err)
	}
}

// Master serves the multivariant playlist.
func (l *Live) Master(w http.ResponseWriter, r *http.Request) {
	m, err := l.Channel.Master(r.Context(), r.URL.Query().Get("lang"))
	if err != nil {
		failed(w, err)
		return
	}
	writePlaylist(w, m.String())
}

var playlistRe = regexp.MustCompile(`^(video|audio-[a-z]{2}|subs-[a-z]{2})\.m3u8$`)

// Playlist serves a rendition's live media playlist.
func (l *Live) Playlist(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "playlist")
	if !playlistRe.MatchString(name) {
		httpx.NotFound(w)
		return
	}
	pl, err := l.Channel.Playlist(r.Context(), strings.TrimSuffix(name, ".m3u8"), l.Channel.now())
	if err != nil {
		failed(w, err)
		return
	}
	writePlaylist(w, pl.String())
}

var (
	segmentRe  = regexp.MustCompile(`^(init\.mp4|[A-Za-z0-9_-]{1,64}\.m4s)$`)
	subtitleRe = regexp.MustCompile(`^\d{1,5}\.vtt$`)
	loopRe     = regexp.MustCompile(`^(init\.mp4|\d{1,6}\.m4s)$`)
)

// AiringFile serves an init section, a segment or a subtitle segment of what an
// airing airs: its first program or break. Only an airing that went on air,
// within airingReach of its time on air, is served, and only from the renditions
// its feed names, so the grant reaches nothing else of the library.
func (l *Live) AiringFile(w http.ResponseWriter, r *http.Request) {
	l.airingFile(w, r, 0)
}

// AiringPartFile is AiringFile for the airing's media part named in the path
// (airingBase): the sections of a break with bulletin slots after its first.
func (l *Live) AiringPartFile(w http.ResponseWriter, r *http.Request) {
	k, err := strconv.Atoi(chi.URLParam(r, "part"))
	if err != nil || k < 0 || k > maxParts {
		httpx.NotFound(w)
		return
	}
	l.airingFile(w, r, k)
}

// maxParts bounds a part's index in a path: a break has a few sections at most.
const maxParts = 99

func (l *Live) airingFile(w http.ResponseWriter, r *http.Request, k int) {
	id, track, file := httpx.UUID(r, "airing"), chi.URLParam(r, "track"), chi.URLParam(r, "file")
	if id == "" {
		httpx.NotFound(w)
		return
	}
	a, err := l.Channel.pinnedAiring(r.Context(), id)
	if err != nil {
		httpx.StoreErr(w, err)
		return
	}
	now := l.Channel.now()
	if now.Before(a.Feed.Start.Add(-airingReach)) || now.After(a.Feed.End().Add(airingReach)) {
		httpx.Error(w, http.StatusNotFound, "off_air", "the airing is not on air")
		return
	}
	part, ok := a.Feed.mediaPart(k)
	if !ok {
		httpx.NotFound(w)
		return
	}
	if lang, ok := strings.CutPrefix(track, "subs-"); ok {
		l.subtitle(w, r, part, lang, file)
		return
	}
	dir := part.Video
	if lang, ok := strings.CutPrefix(track, "audio-"); ok {
		dir = part.Audio[lang]
	} else if track != videoKey {
		dir = ""
	}
	if dir == "" || !segmentRe.MatchString(file) {
		httpx.NotFound(w)
		return
	}
	base := l.media.resolve(part.Path)
	if part.Kind == PartProgram {
		base = playback.PackageDir(l.dataDir, part.File)
	}
	l.serveSegment(w, r, filepath.Join(base, dir), file, track != videoKey)
}

// serveSegment writes a file of a rendition directory, cut to ?keep= seconds
// when the playlist lists it trimmed.
func (l *Live) serveSegment(w http.ResponseWriter, r *http.Request, dir, file string, audio bool) {
	w.Header().Set("Content-Type", "video/mp4")
	if audio {
		w.Header().Set("Content-Type", "audio/mp4")
	}
	w.Header().Set("Cache-Control", "private, max-age=3600")
	path := filepath.Join(dir, file)
	keep, err := keepSeconds(r)
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	if keep == 0 {
		httpx.ServeFile(w, r, path)
		return
	}
	initData, err := os.ReadFile(filepath.Join(dir, "init.mp4"))
	if err != nil {
		httpx.NotFound(w)
		return
	}
	in, err := mp4.ParseInit(initData)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		httpx.NotFound(w)
		return
	}
	cut, err := mp4.Trim(data, in, keep)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	w.Write(cut)
}

func keepSeconds(r *http.Request) (float64, error) {
	raw := r.URL.Query().Get("keep")
	if raw == "" {
		return 0, nil
	}
	keep, err := strconv.ParseFloat(raw, 64)
	if err != nil || keep <= 0 || keep > 60 {
		return 0, errors.New("keep must be a number of seconds")
	}
	return keep, nil
}

// subtitle serves segment n of a program's subtitles on the 6 s grid.
func (l *Live) subtitle(w http.ResponseWriter, r *http.Request, part LivePart, lang, file string) {
	id := part.Subtitles[lang]
	if id == "" || !subtitleRe.MatchString(file) {
		httpx.NotFound(w)
		return
	}
	n, _ := strconv.Atoi(strings.TrimSuffix(file, ".vtt"))
	start := float64(n) * subtitleSeconds
	if start >= part.duration().Seconds() {
		httpx.NotFound(w)
		return
	}
	path, err := l.store.SubtitlePath(r.Context(), id)
	if err != nil {
		httpx.StoreErr(w, err)
		return
	}
	vtt, err := l.vtt.get(filepath.Join(l.dataDir, path))
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "subtitle_unreadable", err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	io.WriteString(w, vtt.Segment(start, start+subtitleSeconds, subtitleMPEGTS))
}

// SlateFile serves the slate: its tracks' init sections, copy n of their
// segment, and the empty WebVTT segment.
func (l *Live) SlateFile(w http.ResponseWriter, r *http.Request) {
	track, file := chi.URLParam(r, "track"), chi.URLParam(r, "file")
	if track == "subs" && file == "empty.vtt" {
		w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		io.WriteString(w, emptyWebVTT)
		return
	}
	if l.Slate.Ensure(r.Context()) != nil {
		httpx.Error(w, http.StatusNotFound, "not_found", "the slate is unavailable")
		return
	}
	serveLoop(w, r, l.Slate.Fill(), track, file)
}

// StandbyFile serves the standby card looped: its tracks' init sections and copy
// n of their segments. The card is a channel asset, so any channel grant reaches it.
func (l *Live) StandbyFile(w http.ResponseWriter, r *http.Request) {
	id, track, file := httpx.UUID(r, "clip"), chi.URLParam(r, "track"), chi.URLParam(r, "file")
	if id == "" {
		httpx.NotFound(w)
		return
	}
	card, err := l.store.Clip(r.Context(), id)
	if err != nil {
		httpx.StoreErr(w, err)
		return
	}
	if card.Kind != ClipStandby || card.State != ClipReady {
		httpx.NotFound(w)
		return
	}
	f, err := l.media.Fill(card.Path)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "not_found", "the standby card is unreadable")
		return
	}
	serveLoop(w, r, f, track, file)
}

// serveLoop writes a filler's init section or copy n of its segments.
func serveLoop(w http.ResponseWriter, r *http.Request, f *fill, track, file string) {
	loop := f.Video
	if track == "audio" {
		loop = f.Audio
	} else if track != "video" {
		loop = nil
	}
	if loop == nil || loop.in == nil || !loopRe.MatchString(file) {
		httpx.NotFound(w)
		return
	}
	keep, err := keepSeconds(r)
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	data := loop.init
	if file != "init.mp4" {
		n, _ := strconv.Atoi(strings.TrimSuffix(file, ".m4s"))
		if data, err = loop.copy(n, keep); err != nil {
			httpx.Internal(w, err)
			return
		}
	}
	w.Header().Set("Content-Type", "video/mp4")
	if track == "audio" {
		w.Header().Set("Content-Type", "audio/mp4")
	}
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.Write(data)
}

// vttCache keeps recently parsed subtitle files: a player asks for the same
// track once per segment.
type vttCache struct {
	mu      sync.Mutex
	entries map[string]vttEntry
}

type vttEntry struct {
	mtime time.Time
	vtt   *hls.WebVTT
}

func (c *vttCache) get(path string) (*hls.WebVTT, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.entries[path]; ok && e.mtime.Equal(st.ModTime()) {
		return e.vtt, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	vtt, err := hls.ParseWebVTT(string(data))
	if err != nil {
		return nil, err
	}
	if c.entries == nil || len(c.entries) >= 16 {
		c.entries = map[string]vttEntry{}
	}
	c.entries[path] = vttEntry{mtime: st.ModTime(), vtt: vtt}
	return vtt, nil
}
