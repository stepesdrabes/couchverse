package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"couchverse/internal/config"
	"couchverse/internal/feature/jobs"
	"couchverse/internal/feature/playback"
	"couchverse/internal/hls"
	"couchverse/internal/media"
	"couchverse/internal/media/mp4"
)

// spikeItem is one program or break of the spike's channel: its package's
// renditions under the channel's rendition keys, and what the browser checks
// expect of it (the tone each audio language plays, the subtitle cues).
type spikeItem struct {
	name     string
	kind     string // program, break
	fileID   string
	duration time.Duration
	video    playback.PackageRendition
	audio    map[string]playback.PackageRendition // by channel language
	subs     map[string]*hls.WebVTT
	tones    map[string]int
}

// spikeChannel airs a loop of items from start, numbered from zero there.
type spikeChannel struct {
	dataDir string
	loop    []*spikeItem
	start   time.Time
	target  int
	window  time.Duration
	// trim serves the segments that run past their item cut short
	trim bool
	// entries are the aired items in order, each with its start and numbering
	entries []spikeEntry
}

type spikeEntry struct {
	item   *spikeItem
	start  time.Time
	origin hls.LiveOrigin
}

var spikeLanguages = []string{"cs", "en"}

// TestLiveSpike is spike S1 of Couch TV (docs/couch-tv-plan.md): it prepares
// `make sample-media` clips, a sparse-keyframe 1080p source and two break clips
// through the real probe and transcode jobs, stitches their packages into a
// looping live schedule with the hls kernel, serves it, and checks it with the
// HLS validator right before and after item boundaries, with hls.js in
// Chromium and WebKit (clients/web/e2e/live.mjs, which also measures stalls,
// alignment and sync) and, on macOS, with AVFoundation. It runs for a few
// minutes and needs COUCHVERSE_LIVE_SPIKE=1, ffmpeg and clients/web's
// dependencies. COUCHVERSE_SAMPLES reuses a `make sample-media` directory
// (absolute path), COUCHVERSE_SPIKE_OUT keeps the playlist snapshots and the
// browsers' samples, COUCHVERSE_SPIKE_SECONDS sets how long the players play,
// COUCHVERSE_SPIKE_TRIM=0 lists audio that runs past a program uncut,
// COUCHVERSE_AVPLAYER=0 skips AVFoundation and VIEWERS picks the browser
// viewers (e2e/live.mjs).
func TestLiveSpike(t *testing.T) {
	if os.Getenv("COUCHVERSE_LIVE_SPIKE") != "1" {
		t.Skip("set COUCHVERSE_LIVE_SPIKE=1 to run the live stitching spike")
	}
	for _, tool := range []string{"ffmpeg", "ffprobe", "node"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not installed", tool)
		}
	}
	env := newTestEnv(t, func(cfg *config.Config) { cfg.FFmpegPath, cfg.FFprobePath = "ffmpeg", "ffprobe" })
	ctx := context.Background()
	w := newWorker(t, env)
	movies := filepath.Join(env.cfg.DataDir, "media", "movies")

	samples := os.Getenv("COUCHVERSE_SAMPLES")
	if samples == "" {
		samples = filepath.Join(t.TempDir(), "samples")
		if out, err := exec.Command("../../../scripts/gen-sample-media.sh", samples, "40").CombinedOutput(); err != nil {
			t.Fatalf("sample media: %v\n%s", err, out)
		}
	}
	ingest := func(title, file string) string {
		dst := filepath.Join(movies, title, file)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Link(filepath.Join(samples, "movies", title, file), dst); err != nil {
			t.Fatal(err)
		}
		return w.ingest(ctx, dst)
	}
	// a copied 720p H.264 source with English and Czech subtitles, and an HDR10
	// HEVC source the browser gets as a transcoded rung, in English and Czech
	harbor := ingest("Glass Harbor (2025)", "Glass Harbor (2025).mkv")
	bloom := ingest("Static Bloom (2024)", "Static Bloom (2024).mkv")
	// a copied 1080p30 source with a keyframe every 10.5 s and one untagged track
	night := spikeClip(t, w, movies, "Night Shift (2026)", "Night Shift (2026).mkv",
		"testsrc2=size=1920x1080:rate=30:duration=40", 550, 40,
		"-c:v", "libx264", "-preset", "veryfast", "-x264-params", "keyint=315:min-keyint=315:scenecut=0", "-c:a", "aac")
	// breaks as the renderer will make them: 1080p25 H.264 High, 2 s GOP, AAC
	breakArgs := []string{"-vf", "drawbox=x='mod(t*240,1700)':y=80:w=160:h=160:color=white:t=fill",
		"-c:v", "libx264", "-preset", "veryfast", "-profile:v", "high", "-crf", "18", "-g", "50", "-keyint_min", "50",
		"-sc_threshold", "0", "-pix_fmt", "yuv420p", "-c:a", "aac", "-b:a", "192k", "-ac", "2"}
	break1 := spikeClip(t, w, movies, "Break One (2026)", "break1.mp4", "color=c=0x1040c0:size=1920x1080:rate=25:duration=17.36", 880, 17.36, breakArgs...)
	break2 := spikeClip(t, w, movies, "Break Two (2026)", "break2.mp4", "color=c=0x1040c0:size=1920x1080:rate=25:duration=12.48", 880, 12.48, breakArgs...)
	w.drain(ctx)
	// a lone H.264/AAC MP4 plays directly, so the breaks' packages are asked for
	for _, id := range []string{break1, break2} {
		if _, err := w.jobs.EnqueueJob(ctx, "transcode_hls", playback.Payload{MediaFileID: id, Variant: media.VariantPackage}, jobs.EnqueueOpts{}); err != nil {
			t.Fatal(err)
		}
	}
	w.drain(ctx)

	item := func(name, kind, fileID string, ladder bool, tones map[string]int, airFor float64) *spikeItem {
		p, err := playback.ReadPackage(env.cfg.DataDir, fileID)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		it := &spikeItem{name: name, kind: kind, fileID: fileID, audio: map[string]playback.PackageRendition{},
			subs: map[string]*hls.WebVTT{}, tones: tones}
		switch {
		case ladder && len(p.Ladder) > 0:
			it.video = p.Ladder[0]
		case !ladder && p.Source != nil:
			it.video = *p.Source
		default:
			t.Fatalf("%s: no video rendition in %+v", name, p)
		}
		if len(p.Stereo) == 0 {
			t.Fatalf("%s: no audio", name)
		}
		// each language gets its own track, else the original (the default one)
		original := p.Stereo[0]
		for _, r := range p.Stereo {
			if r.Info.Default {
				original = r
				break
			}
		}
		for _, lang := range spikeLanguages {
			it.audio[lang] = original
			for _, r := range p.Stereo {
				if r.Info.Language == lang {
					it.audio[lang] = r
				}
			}
		}
		subs, err := w.subtitles.Subs.SubtitlesForMediaFile(ctx, fileID)
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range subs {
			data, err := os.ReadFile(filepath.Join(env.cfg.DataDir, s.Path))
			if err != nil {
				t.Fatal(err)
			}
			vtt, err := hls.ParseWebVTT(string(data))
			if err != nil {
				t.Fatal(err)
			}
			it.subs[media.BCP47(s.Lang)] = vtt
		}
		onAir := airFor
		if onAir == 0 {
			onAir = it.video.Info.Duration
		}
		it.duration = time.Duration(hls.Cut(it.video.Playlist.Segments, onAir) * float64(time.Second))
		t.Logf("%s: %s %dx%d %.3f fps, %d video segments, airs %s, audio %v, subtitles %v", name, it.video.Info.Codec,
			it.video.Info.Width, it.video.Info.Height, it.video.Info.FrameRate, len(it.video.Playlist.Segments), it.duration,
			languages(it.audio), slices.Sorted(maps.Keys(it.subs)))
		return it
	}
	b1 := item("Break One", "break", break1, false, map[string]int{"cs": 880, "en": 880}, 0)
	b2 := item("Break Two", "break", break2, false, map[string]int{"cs": 880, "en": 880}, 0)
	ch := &spikeChannel{
		dataDir: env.cfg.DataDir,
		loop: []*spikeItem{
			item("Glass Harbor", "program", harbor, false, map[string]int{"cs": 330, "en": 330}, 0),
			b1,
			item("Static Bloom", "program", bloom, true, map[string]int{"cs": 440, "en": 330}, 0),
			b2,
			// cut on the video boundary at 31.5 s, which no audio boundary matches
			item("Night Shift", "program", night, false, map[string]int{"cs": 550, "en": 550}, 30),
			b1,
		},
		window: 2 * time.Minute,
		trim:   os.Getenv("COUCHVERSE_SPIKE_TRIM") != "0",
	}
	for _, it := range ch.loop {
		ch.target = max(ch.target, hls.TargetDuration(ch.liveItem(0, it).MaxSegment()))
	}
	// the channel has been on air for a while, so players join mid-item
	ch.start = time.Now().Add(-95 * time.Second).Truncate(time.Millisecond)
	hold, _ := time.ParseDuration(os.Getenv("COUCHVERSE_SPIKE_HOLD"))
	ch.extend(time.Now().Add(30*time.Minute + hold))

	srv := httptest.NewUnstartedServer(ch)
	if addr := os.Getenv("COUCHVERSE_SPIKE_ADDR"); addr != "" {
		l, err := net.Listen("tcp", addr)
		if err != nil {
			t.Fatal(err)
		}
		srv.Listener.Close()
		srv.Listener = l
	}
	srv.Start()
	defer srv.Close()
	master := srv.URL + "/live/master.m3u8"
	t.Logf("channel on %s (target duration %ds), a player on %s/watch.html", master, ch.target, srv.URL)

	// the browsers and AVFoundation play while snapshots are validated
	var wg sync.WaitGroup
	results := map[string]string{}
	var mu sync.Mutex
	run := func(name string, cmd *exec.Cmd) {
		defer wg.Done()
		out, err := cmd.CombinedOutput()
		mu.Lock()
		defer mu.Unlock()
		results[name] = string(out)
		if err != nil {
			t.Errorf("%s: %v\n%s", name, err, out)
		}
	}
	seconds := os.Getenv("COUCHVERSE_SPIKE_SECONDS")
	if seconds == "" {
		seconds = "130"
	}
	// the players stop with the test, whatever ends it
	players, stop := context.WithCancel(ctx)
	defer stop()
	browsers := exec.CommandContext(players, "node", "e2e/live.mjs")
	browsers.Dir = "../../../clients/web"
	browsers.Env = append(os.Environ(), "SERVER="+srv.URL, "SECONDS="+seconds)
	if out := os.Getenv("COUCHVERSE_SPIKE_OUT"); out != "" {
		browsers.Env = append(browsers.Env, "OUT="+filepath.Join(out, "browsers.json"))
	}
	wg.Add(1)
	go run("hls.js", browsers)
	if runtime.GOOS == "darwin" && os.Getenv("COUCHVERSE_AVPLAYER") != "0" {
		wg.Add(1)
		go run("AVFoundation", exec.CommandContext(players, "swift", "../../../scripts/avplayer-probe.swift", master,
			"--live", seconds, "--audio", "2", "--subtitles", "2"))
	}
	ch.validateBoundaries(t, srv, 4)
	wg.Wait()
	for _, name := range []string{"hls.js", "AVFoundation"} {
		if out, ok := results[name]; ok {
			t.Logf("%s:\n%s", name, out)
		}
	}
	if out, ok := results["AVFoundation"]; ok {
		ch.checkAVPlayer(t, out)
	}
	// a device that cannot run the checks (the Titan OS TV) watches meanwhile
	if hold > 0 {
		t.Logf("serving %s/watch.html for %s", srv.URL, hold)
		time.Sleep(hold)
	}
}

func languages(audio map[string]playback.PackageRendition) string {
	var out []string
	for _, lang := range spikeLanguages {
		out = append(out, lang+"="+audio[lang].Name+"("+audio[lang].Info.Language+")")
	}
	return strings.Join(out, " ")
}

// spikeClip generates a clip from a lavfi video source and a sine tone and ingests it.
func spikeClip(t *testing.T, w *worker, movies, title, file, video string, tone int, seconds float64, args ...string) string {
	t.Helper()
	path := filepath.Join(movies, title, file)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	d := strconv.FormatFloat(seconds, 'f', -1, 64)
	cmd := append([]string{"-y", "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", video,
		"-f", "lavfi", "-i", fmt.Sprintf("sine=frequency=%d:duration=%s:sample_rate=48000", tone, d),
		"-map", "0:v", "-map", "1:a"}, args...)
	if out, err := exec.Command("ffmpeg", append(cmd, path)...).CombinedOutput(); err != nil {
		t.Fatalf("generate %s: %v %s", file, err, out)
	}
	return w.ingest(context.Background(), path)
}

// liveItem is an item's renditions as the channel's playlists list them: the
// package's segments under /pkg, subtitles cut along a 6 s grid (empty WebVTT
// where the item has none in a language).
func (c *spikeChannel) liveItem(index int, it *spikeItem) hls.LiveItem {
	pkg := func(r playback.PackageRendition) []hls.Segment {
		base := "/pkg/" + it.fileID + "/" + r.Name + "/"
		out := slices.Clone(r.Playlist.Segments)
		for i := range out {
			out[i].URI = base + out[i].URI
			m := *out[i].Map
			m.URI = base + m.URI
			out[i].Map = &m
		}
		return out
	}
	li := hls.LiveItem{Duration: it.duration, Renditions: map[string][]hls.Segment{"video": pkg(it.video)}}
	for _, lang := range spikeLanguages {
		li.Renditions["audio-"+lang] = pkg(it.audio[lang])
		var subs []hls.Segment
		for k := 0; float64(k)*6 < it.duration.Seconds(); k++ {
			uri := "/live/empty.vtt"
			if it.subs[lang] != nil {
				uri = fmt.Sprintf("/live/subs/%d/%s/%d.vtt", index, lang, k)
			}
			subs = append(subs, hls.Segment{Duration: min(6, it.duration.Seconds()-float64(k)*6), URI: uri})
		}
		li.Renditions["subs-"+lang] = subs
	}
	return li
}

// extend airs the loop until the given time, chaining every item's numbering.
func (c *spikeChannel) extend(until time.Time) {
	start, origin := c.start, hls.LiveOrigin{MediaSequence: map[string]int{}}
	for i := 0; start.Before(until); i++ {
		it := c.loop[i%len(c.loop)]
		c.entries = append(c.entries, spikeEntry{item: it, start: start, origin: origin})
		origin = origin.After(c.liveItem(i, it))
		start = start.Add(it.duration)
	}
}

// schedule is the part of the channel a playlist at now needs: from the item
// on air when its window opens, so the request is as cheap as the feature's.
func (c *spikeChannel) schedule(now time.Time) *hls.LiveSchedule {
	opens := now.Add(-max(c.window, time.Duration(4*c.target)*time.Second))
	first := 0
	for i, e := range c.entries {
		if !e.start.After(opens) {
			first = i
		}
	}
	s := &hls.LiveSchedule{Start: c.entries[first].start, Origin: c.entries[first].origin,
		TargetDuration: c.target, Window: c.window}
	if c.trim {
		s.Trim = func(uri string, keep float64) string { return fmt.Sprintf("%s?keep=%.3f", uri, keep) }
	}
	for i := first; i < len(c.entries) && c.entries[i].start.Before(now.Add(time.Minute)); i++ {
		s.Items = append(s.Items, c.liveItem(i, c.entries[i].item))
	}
	return s
}

// spikeWatchPage plays the channel with hls.js, holding the program date the
// way the web player will, and shows the date, latency and audio choices so a
// TV can be checked by eye against a laptop.
const spikeWatchPage = `<!doctype html>
<html><body style="margin:0;background:#000;color:#fff;font:24px sans-serif">
<video id="v" style="width:100vw;height:100vh" autoplay playsinline></video>
<div id="o" style="position:fixed;left:24px;top:24px;background:#0008;padding:8px 12px"></div>
<script src="/hls.min.js"></script>
<script>
const video = document.getElementById('v'), overlay = document.getElementById('o');
const target = 3 * TARGET, hls = new Hls({ liveSyncDuration: target, liveMaxLatencyDuration: target + 2 * TARGET });
let offset = 0, events = [];
fetch('/clock').then((r) => r.json()).then(({ now }) => (offset = now - Date.now()));
hls.on(Hls.Events.ERROR, (_, d) => events.push(d.details + (d.fatal ? ' (fatal)' : '')));
hls.loadSource('/live/master.m3u8?lang=' + (new URLSearchParams(location.search).get('lang') || 'cs'));
hls.attachMedia(video);
video.addEventListener('waiting', () => events.push('waiting at ' + video.currentTime.toFixed(1)));
// left and right switch audio, up and down subtitles (remote arrows)
addEventListener('keydown', (e) => {
  const n = (i, len, d) => (i + d + len + 1) % (len + 1) - 1;
  if (e.key === 'ArrowLeft' || e.key === 'ArrowRight') hls.audioTrack = (hls.audioTrack + 1) % hls.audioTracks.length;
  if (e.key === 'ArrowUp' || e.key === 'ArrowDown') { hls.subtitleDisplay = true; hls.subtitleTrack = n(hls.subtitleTrack, hls.subtitleTracks.length, 1); }
  video.play();
});
setInterval(() => {
  if (!hls.playingDate) return;
  const drift = (hls.playingDate.getTime() - (Date.now() + offset - target * 1000)) / 1000;
  if (Math.abs(drift) > 2) { video.playbackRate = 1; video.currentTime -= drift; }
  else video.playbackRate = Math.abs(drift) < 0.1 ? 1 : 1 - Math.max(-0.1, Math.min(0.1, drift / 5));
  const audio = hls.audioTracks[hls.audioTrack], subs = hls.subtitleTracks[hls.subtitleTrack];
  overlay.textContent = hls.playingDate.toISOString().slice(11, 23) + ' UTC, drift ' + drift.toFixed(2) +
    's, ' + video.videoWidth + 'x' + video.videoHeight + ', audio ' + (audio ? audio.lang : '-') +
    ', subtitles ' + (subs ? subs.lang : 'off') + (events.length ? ', ' + events.slice(-3).join(', ') : '');
}, 500);
</script></body></html>`

var spikeSubtitle = regexp.MustCompile(`^/live/subs/(\d+)/([a-z]+)/(\d+)\.vtt$`)

func (c *spikeChannel) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	path := r.URL.Path
	switch {
	case path == "/live/master.m3u8":
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Header().Set("Cache-Control", "no-cache")
		io.WriteString(w, c.master(r.URL.Query().Get("lang")).String())
	case strings.HasPrefix(path, "/live/") && strings.HasSuffix(path, ".m3u8"):
		key := strings.TrimSuffix(strings.TrimPrefix(path, "/live/"), ".m3u8")
		pl, err := c.schedule(now).Playlist(key, now)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Header().Set("Cache-Control", "no-cache")
		io.WriteString(w, pl.String())
	case path == "/live/empty.vtt":
		w.Header().Set("Content-Type", "text/vtt")
		io.WriteString(w, (&hls.WebVTT{}).Segment(0, 0, 126000))
	case spikeSubtitle.MatchString(path):
		m := spikeSubtitle.FindStringSubmatch(path)
		index, _ := strconv.Atoi(m[1])
		k, _ := strconv.Atoi(m[3])
		if index >= len(c.entries) || c.entries[index].item.subs[m[2]] == nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/vtt")
		io.WriteString(w, c.entries[index].item.subs[m[2]].Segment(float64(k)*6, float64(k+1)*6, 126000))
	case strings.HasPrefix(path, "/pkg/"):
		rel := filepath.Clean(strings.TrimPrefix(path, "/pkg/"))
		if strings.HasPrefix(rel, "..") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "video/mp4")
		path := filepath.Join(c.dataDir, "cache", "hls", rel)
		if keep := r.URL.Query().Get("keep"); keep != "" {
			seconds, _ := strconv.ParseFloat(keep, 64)
			if err := serveTrimmed(w, path, seconds); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		http.ServeFile(w, r, path)
	case path == "/watch.html":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		io.WriteString(w, strings.ReplaceAll(spikeWatchPage, "TARGET", strconv.Itoa(c.target)))
	case path == "/hls.min.js":
		w.Header().Set("Content-Type", "text/javascript")
		http.ServeFile(w, r, "../../../clients/web/node_modules/hls.js/dist/hls.min.js")
	case path == "/clock":
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"now":%d}`, now.UnixMilli())
	case path == "/schedule.json":
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(c.describe()); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	default:
		http.NotFound(w, r)
	}
}

// serveTrimmed writes a segment cut to its first seconds, read against the
// init section next to it.
func serveTrimmed(w http.ResponseWriter, path string, seconds float64) error {
	initData, err := os.ReadFile(filepath.Join(filepath.Dir(path), "init.mp4"))
	if err != nil {
		return err
	}
	in, err := mp4.ParseInit(initData)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	cut, err := mp4.Trim(data, in, seconds)
	if err != nil {
		return err
	}
	_, err = w.Write(cut)
	return err
}

// master offers the one variant covering every item, Czech and English audio
// and subtitles.
func (c *spikeChannel) master(lang string) *hls.Multivariant {
	var codecs []string
	video := hls.Variant{VideoRange: "SDR", URI: "video.m3u8"}
	var audioPeak, audioAverage, videoPeak, videoAverage int64
	for _, it := range c.loop {
		v := it.video.Info
		codecs = append(codecs, v.Codec)
		video.Width, video.Height = max(video.Width, v.Width), max(video.Height, v.Height)
		video.FrameRate = max(video.FrameRate, v.FrameRate)
		videoPeak, videoAverage = max(videoPeak, v.Peak), max(videoAverage, v.Average)
		for _, a := range it.audio {
			audioPeak, audioAverage = max(audioPeak, a.Info.Peak), max(audioAverage, a.Info.Average)
		}
	}
	video.Codecs = hls.CoverCodecs(append(codecs, "mp4a.40.2"))
	video.Bandwidth, video.AverageBandwidth = videoPeak+audioPeak, videoAverage+audioAverage
	names := map[string]string{"cs": "Čeština", "en": "English"}
	var audio, subs []hls.Rendition
	for _, l := range spikeLanguages {
		audio = append(audio, hls.Rendition{Name: names[l], Language: l, Channels: "2", URI: "audio-" + l + ".m3u8"})
		subs = append(subs, hls.Rendition{Name: names[l], Language: l, URI: "subs-" + l + ".m3u8"})
	}
	return hls.LiveMultivariant(video, audio, subs, lang)
}

type spikeCue struct {
	Start int64  `json:"start"`
	End   int64  `json:"end"`
	Text  string `json:"text"`
}

type spikeScheduleItem struct {
	Name      string                `json:"name"`
	Kind      string                `json:"kind"`
	Start     int64                 `json:"start"`
	End       int64                 `json:"end"`
	Tones     map[string]int        `json:"tones"`
	Subtitles map[string][]spikeCue `json:"subtitles"`
	Video     map[string]any        `json:"video"`
}

// describe is the schedule as the browser checks read it, in Unix milliseconds.
func (c *spikeChannel) describe() map[string]any {
	var items []spikeScheduleItem
	for _, e := range c.entries {
		it := e.item
		s := spikeScheduleItem{Name: it.name, Kind: it.kind, Start: e.start.UnixMilli(),
			End: e.start.Add(it.duration).UnixMilli(), Tones: it.tones, Subtitles: map[string][]spikeCue{},
			Video: map[string]any{"width": it.video.Info.Width, "height": it.video.Info.Height, "codec": it.video.Info.Codec}}
		for lang, vtt := range it.subs {
			for _, cue := range vtt.Cues {
				if cue.Start < it.duration.Seconds() {
					s.Subtitles[lang] = append(s.Subtitles[lang], spikeCue{
						Start: e.start.Add(time.Duration(cue.Start * float64(time.Second))).UnixMilli(),
						End:   e.start.Add(time.Duration(cue.End * float64(time.Second))).UnixMilli(), Text: cue.Payload})
				}
			}
		}
		items = append(items, s)
	}
	return map[string]any{"items": items, "targetDuration": c.target}
}

// validateBoundaries validates the presentation just before and just after
// the next n item boundaries.
func (c *spikeChannel) validateBoundaries(t *testing.T, srv *httptest.Server, n int) {
	out := os.Getenv("COUCHVERSE_SPIKE_OUT")
	var boundaries []time.Time
	for _, e := range c.entries {
		if e.start.After(time.Now().Add(3*time.Second)) && len(boundaries) < n {
			boundaries = append(boundaries, e.start)
		}
	}
	for i, b := range boundaries {
		for _, at := range []struct {
			name string
			time time.Time
		}{{"before", b.Add(-1500 * time.Millisecond)}, {"after", b.Add(1500 * time.Millisecond)}} {
			time.Sleep(time.Until(at.time))
			name := fmt.Sprintf("boundary %d %s", i+1, at.name)
			report, err := hls.Validate(context.Background(), hls.HTTPFetcher{Client: srv.Client()}, srv.URL+"/live/master.m3u8", hls.Options{})
			if err != nil {
				t.Errorf("%s: %v", name, err)
				continue
			}
			warnings := map[string]int{}
			for _, issue := range report.Issues {
				if issue.Severity == hls.SeverityError {
					t.Errorf("%s: %s", name, issue)
				} else {
					warnings[issue.Rule]++
				}
			}
			t.Logf("%s (%s): %d playlists, %d segments, %d errors, warnings %v", name, hls.FormatDateTime(at.time),
				report.Playlists, report.Segments, len(report.Errors()), warnings)
			if out != "" {
				c.snapshot(t, srv, filepath.Join(out, fmt.Sprintf("boundary-%d-%s", i+1, at.name)))
			}
		}
	}
}

// snapshot keeps every playlist of the presentation as served right now. It
// runs while the players play, so a failure is reported rather than fatal.
func (c *spikeChannel) snapshot(t *testing.T, srv *httptest.Server, dir string) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Errorf("snapshot: %v", err)
		return
	}
	for _, name := range []string{"master", "video", "audio-cs", "audio-en", "subs-cs", "subs-en"} {
		res, err := srv.Client().Get(srv.URL + "/live/" + name + ".m3u8")
		if err != nil {
			t.Errorf("snapshot: %v", err)
			return
		}
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if err := os.WriteFile(filepath.Join(dir, name+".m3u8"), body, 0o644); err != nil {
			t.Errorf("snapshot: %v", err)
			return
		}
	}
}

var avDate = regexp.MustCompile(`(?m)^  t=([\d.]+) time=([\d.]+) date=(\d+) stalls=(\d+)`)

// checkAVPlayer reads the AVFoundation probe's live log: the program date must
// advance with the clock across at least three item boundaries.
func (c *spikeChannel) checkAVPlayer(t *testing.T, out string) {
	samples := avDate.FindAllStringSubmatch(out, -1)
	if len(samples) < 10 {
		t.Errorf("AVFoundation: only %d samples", len(samples))
		return
	}
	parse := func(s []string) (float64, int64, int) {
		elapsed, _ := strconv.ParseFloat(s[1], 64)
		date, _ := strconv.ParseInt(s[3], 10, 64)
		stalls, _ := strconv.Atoi(s[4])
		return elapsed, date, stalls
	}
	firstAt, firstDate, _ := parse(samples[len(samples)/10])
	lastAt, lastDate, stalls := parse(samples[len(samples)-1])
	drift := float64(lastDate-firstDate)/1000 - (lastAt - firstAt)
	crossed := 0
	for _, e := range c.entries {
		if ms := e.start.UnixMilli(); ms > firstDate && ms < lastDate {
			crossed++
		}
	}
	t.Logf("AVFoundation: dates %s to %s, %d boundaries, drift %.2fs, %d stalls",
		hls.FormatDateTime(time.UnixMilli(firstDate)), hls.FormatDateTime(time.UnixMilli(lastDate)), crossed, drift, stalls)
	if crossed < 3 || drift < -1.5 || drift > 1.5 || stalls > 0 {
		t.Errorf("AVFoundation: %d boundaries crossed, %.2fs behind the clock, %d stalls", crossed, -drift, stalls)
	}
}
