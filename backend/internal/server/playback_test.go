package server_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"couchverse/internal/config"
	"couchverse/internal/db"
	"couchverse/internal/feature/catalog"
	"couchverse/internal/feature/downloads"
	"couchverse/internal/feature/jobs"
	"couchverse/internal/feature/library"
	"couchverse/internal/feature/playback"
	"couchverse/internal/feature/subtitles"
	"couchverse/internal/hls"
	"couchverse/internal/media"
	"couchverse/internal/settings"
)

// Device profiles as the clients send them (contract/fixtures/device-profiles).
var (
	chromeProfile = deviceProfile("chrome-desktop")
	appleProfile  = deviceProfile("apple-tv-4k")
)

func deviceProfile(name string) json.RawMessage {
	data, err := os.ReadFile("../../../contract/fixtures/device-profiles/" + name + ".json")
	if err != nil {
		panic(err)
	}
	return data
}

// sample is one generated source file and how each device must get it.
type sample struct {
	title string // "Name (Year)", also the movie's draft title
	file  string
	args  []string // ffmpeg arguments after the lavfi inputs
	// subs are SRT files muxed in as text subtitles
	subs []string
	// want maps a profile name to the expected tier/mode
	want map[string]string
	// audio and subtitle options AVFoundation must list on the remuxed master
	audio, subtitles int
}

const sampleSeconds = 14

// TestPlaybackTiers prepares generated sample media through the real probe and
// transcode jobs, asks for playback as a browser and as an Apple device, and
// validates every HLS presentation it is given with the HLS validator. With
// COUCHVERSE_AVPLAYER=1 (macOS) each one is also played by AVFoundation through
// scripts/avplayer-probe.swift, and with COUCHVERSE_APPLE_HLS_TOOLS=1 checked by
// Apple's mediastreamvalidator when it is installed. It needs ffmpeg.
func TestPlaybackTiers(t *testing.T) {
	if testing.Short() {
		t.Skip("encodes video")
	}
	for _, tool := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not installed", tool)
		}
	}
	env := newTestEnv(t, func(cfg *config.Config) { cfg.FFmpegPath, cfg.FFprobePath = "ffmpeg", "ffprobe" })
	ctx := context.Background()
	w := newWorker(t, env)

	samples := []sample{
		{
			title: "Test Pattern (2026)", file: "Test Pattern (2026).mp4",
			args: []string{"-map", "0:v", "-map", "1:a", "-c:v", "libx264", "-preset", "ultrafast", "-c:a", "aac"},
			want: map[string]string{"chrome": "direct/direct", "apple": "direct/direct"},
		},
		{
			// MKV with AC-3 5.1 and two subtitle languages: remux everywhere
			title: "Harbor Lights (2025)", file: "Harbor Lights (2025).mkv",
			args: []string{"-map", "0:v", "-map", "2:a", "-c:v", "libx264", "-preset", "ultrafast", "-g", "48",
				"-c:a", "ac3", "-b:a", "384k", "-metadata:s:a:0", "language=eng"},
			subs:  []string{"eng", "ces"},
			want:  map[string]string{"chrome": "remux/hls", "apple": "remux/hls"},
			audio: 1, subtitles: 2,
		},
		{
			// HEVC Main 10 HDR10 with E-AC-3 5.1 and a Czech AAC track: Apple
			// remuxes it as HDR, the browser gets the tone-mapped ladder
			title: "Quiet Meridian (2024)", file: "Quiet Meridian (2024).mkv",
			args: []string{"-map", "0:v", "-map", "2:a", "-map", "1:a",
				"-vf", "format=yuv420p10le,setparams=color_primaries=bt2020:color_trc=smpte2084:colorspace=bt2020nc",
				"-c:v", "libx265", "-preset", "ultrafast", "-x265-params", "keyint=48:min-keyint=48:hdr10=1:log-level=error",
				"-tag:v", "hvc1", "-c:a:0", "eac3", "-b:a:0", "448k", "-c:a:1", "aac", "-ac:a:1", "2",
				"-metadata:s:a:0", "language=eng", "-metadata:s:a:1", "language=ces"},
			subs:  []string{"eng"},
			want:  map[string]string{"chrome": "transcode/hls", "apple": "remux/hls"},
			audio: 2, subtitles: 1,
		},
		{
			// VP9/Opus in WebM direct-plays in a browser but never on Apple
			title: "Neon Drift (2023)", file: "Neon Drift (2023).webm",
			args: []string{"-map", "0:v", "-map", "1:a", "-c:v", "libvpx-vp9", "-deadline", "realtime", "-cpu-used", "8",
				"-b:v", "1M", "-c:a", "libopus"},
			want: map[string]string{"chrome": "direct/direct", "apple": "transcode/hls"},
		},
	}

	moviesDir := filepath.Join(env.cfg.DataDir, "media", "movies")
	for _, s := range samples {
		path := filepath.Join(moviesDir, s.title, s.file)
		generate(t, path, s)
		w.ingest(ctx, path)
	}
	w.drain(ctx)

	member := env.clients[memberName]
	for _, s := range samples {
		titleID := w.titleID(ctx, s.title)
		for profileName, profile := range map[string]json.RawMessage{"chrome": chromeProfile, "apple": appleProfile} {
			name := s.title + "/" + profileName
			status, body, _ := env.request(t, member, "POST", "/playback/movie/"+titleID, profile)
			if status != 200 {
				t.Fatalf("%s: playback %d %s", name, status, body)
			}
			var info playback.PlaybackInfo
			if err := json.Unmarshal(body, &info); err != nil {
				t.Fatal(err)
			}
			if got := info.Tier + "/" + info.Mode; got != s.want[profileName] {
				t.Errorf("%s: got %s, want %s", name, got, s.want[profileName])
				continue
			}
			switch info.Mode {
			case "hls":
				env.checkHLS(t, name, info.StreamURL, s, profileName == "apple")
				if info.HLSURL != "" && info.HLSURL != info.StreamURL {
					env.checkHLS(t, name+" ladder", info.HLSURL, s, false)
				}
			case "direct":
				// only the Apple profile's files are AVFoundation's to open
				if profileName == "apple" {
					env.checkAVPlayer(t, name, info.StreamURL, nil)
				}
			}
		}
	}

	// instant play: nothing prepared for a file a browser cannot decode
	w.setTranscode(ctx, `{"jitEnabled": true, "autoPrepare": false}`)
	jit := sample{title: "Late Signal (2022)", file: "Late Signal (2022).mkv",
		args: []string{"-map", "0:v", "-map", "2:a", "-c:v", "libx265", "-preset", "ultrafast", "-x265-params", "log-level=error",
			"-c:a", "eac3", "-b:a", "384k"},
		subs: []string{"eng"}}
	path := filepath.Join(moviesDir, jit.title, jit.file)
	generate(t, path, jit)
	w.ingest(ctx, path)
	w.drain(ctx)
	status, body, _ := env.request(t, member, "POST", "/playback/movie/"+w.titleID(ctx, jit.title), chromeProfile)
	var info playback.PlaybackInfo
	if err := json.Unmarshal(body, &info); status != 200 || err != nil || info.Mode != "jit" || info.JIT == nil {
		t.Fatalf("jit: %d %s", status, body)
	}
	status, body, _ = env.request(t, member, "POST", "/media/"+info.Grant+"/jit",
		map[string]any{"startAt": 0, "plan": info.JIT})
	var session playback.StreamSession
	if err := json.Unmarshal(body, &session); status != 201 || err != nil {
		t.Fatalf("jit session: %d %s", status, body)
	}
	env.checkHLS(t, "jit", session.PlaylistURL, jit, false)

	// a session belongs to the viewer whose grant opened it, not to the file
	status, body, _ = env.request(t, env.clients["admin"], "POST", "/playback/movie/"+w.titleID(ctx, jit.title), chromeProfile)
	var other playback.PlaybackInfo
	if err := json.Unmarshal(body, &other); status != 200 || err != nil {
		t.Fatalf("admin playback: %d %s", status, body)
	}
	keepalive := func(grant string) int {
		status, _, _ := env.request(t, env.clients[""], "POST", "/media/"+grant+"/jit/"+session.SessionID+"/keepalive", nil)
		return status
	}
	if got := keepalive(other.Grant); got != 404 {
		t.Errorf("another viewer's grant reached the session: %d", got)
	}
	if got := keepalive(info.Grant); got != 204 {
		t.Errorf("the owner's grant: %d, want 204", got)
	}
}

// TestUnrecordedTranscodeFails checks that a package ffmpeg finished but the
// database would not mark ready (as it refused sizes over 2 GiB) leaves its
// variants failed and no output behind, instead of processing forever.
func TestUnrecordedTranscodeFails(t *testing.T) {
	if testing.Short() {
		t.Skip("encodes video")
	}
	for _, tool := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not installed", tool)
		}
	}
	env := newTestEnv(t, func(cfg *config.Config) { cfg.FFmpegPath, cfg.FFprobePath = "ffmpeg", "ffprobe" })
	ctx := context.Background()
	w := newWorker(t, env)

	s := sample{title: "Test Pattern (2026)", file: "Test Pattern (2026).mp4",
		args: []string{"-map", "0:v", "-map", "1:a", "-c:v", "libx264", "-preset", "ultrafast", "-c:a", "aac"}}
	path := filepath.Join(env.cfg.DataDir, "media", "movies", s.title, s.file)
	generate(t, path, s)
	fileID := w.ingest(ctx, path)
	w.drain(ctx)

	if _, err := env.pool.Exec(ctx, `
		CREATE FUNCTION refuse_ready() RETURNS trigger LANGUAGE plpgsql AS $$
			BEGIN RAISE EXCEPTION 'refusing to mark a variant ready'; END $$;
		CREATE TRIGGER refuse_ready BEFORE UPDATE ON transcode_variants
			FOR EACH ROW WHEN (NEW.status = 'ready') EXECUTE FUNCTION refuse_ready();`); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(playback.Payload{MediaFileID: fileID, Variant: media.VariantPackage})
	if err := w.transcode.Handle(ctx, &jobs.Job{Type: "transcode_hls", Payload: payload}, func(int) {}); err == nil {
		t.Fatal("the package was recorded despite the refusal")
	}

	variants, err := w.files.VariantsForMediaFile(ctx, fileID)
	if err != nil {
		t.Fatal(err)
	}
	packaged := 0
	for _, v := range variants {
		if v.Name != media.VariantSource && v.Name != media.VariantAudio {
			continue
		}
		packaged++
		if v.Status != "failed" {
			t.Errorf("%s variant is %s, want failed", v.Name, v.Status)
		}
	}
	if packaged != 2 {
		t.Fatalf("the package filled %d variants, want source and audio", packaged)
	}
	base := filepath.Join(env.cfg.DataDir, "cache", "hls", fileID)
	left, _ := filepath.Glob(filepath.Join(base, "audio-*"))
	if _, err := os.Stat(filepath.Join(base, media.VariantSource)); err == nil {
		left = append(left, media.VariantSource)
	}
	if len(left) > 0 {
		t.Errorf("output left behind: %v", left)
	}
}

// generate writes a sample with lavfi sources: test pattern video, a 440 Hz
// tone in stereo and a 5.1 one, plus SRT subtitle tracks.
func generate(t *testing.T, path string, s sample) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	d := strconv.Itoa(sampleSeconds)
	args := []string{"-y", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "testsrc2=size=1280x720:rate=24:duration=" + d,
		"-f", "lavfi", "-i", "sine=frequency=440:duration=" + d + ":sample_rate=48000",
		"-f", "lavfi", "-i", "sine=frequency=330:duration=" + d + ":sample_rate=48000,pan=5.1|FL=c0|FR=c0|FC=c0|LFE=c0|BL=c0|BR=c0",
	}
	for i, lang := range s.subs {
		srt := filepath.Join(t.TempDir(), lang+".srt")
		cues := fmt.Sprintf("1\n00:00:01,000 --> 00:00:04,000\n%s first cue\n\n2\n00:00:05,500 --> 00:00:07,500\n%s across a segment boundary\n\n3\n00:00:12,000 --> 00:00:13,000\n%s last cue\n", lang, lang, lang)
		if err := os.WriteFile(srt, []byte(cues), 0o644); err != nil {
			t.Fatal(err)
		}
		args = append(args, "-i", srt)
		s.args = append(s.args, "-map", strconv.Itoa(3+i), fmt.Sprintf("-metadata:s:s:%d", i), "language="+lang)
	}
	if len(s.subs) > 0 {
		s.args = append(s.args, "-c:s", "srt")
	}
	// every lavfi input has a duration; -shortest would stall on the subtitle inputs
	args = append(append(args, s.args...), path)
	if out, err := exec.Command("ffmpeg", args...).CombinedOutput(); err != nil {
		t.Fatalf("generate %s: %v %s", s.file, err, out)
	}
}

// worker runs the probe, subtitle and transcode jobs the way the app's runner
// does, one at a time.
type worker struct {
	t         *testing.T
	env       *testEnv
	files     *library.Store
	catalog   *catalog.Store
	jobs      *jobs.Store
	settings  *settings.Store
	prober    *library.Prober
	subtitles *subtitles.Service
	transcode *playback.JobHandler
	downloads *downloads.Preparer
}

func newWorker(t *testing.T, env *testEnv) *worker {
	files, set, jb := library.NewStore(env.pool), settings.NewStore(env.pool), jobs.NewStore(env.pool)
	cat := catalog.NewStore(env.pool)
	subs := &subtitles.Service{Subs: subtitles.NewStore(env.pool), Files: files, DataDir: env.cfg.DataDir, FFmpegPath: "ffmpeg"}
	return &worker{
		t: t, env: env, files: files, catalog: cat, jobs: jb, settings: set,
		prober:    &library.Prober{Files: files, Catalog: cat, Settings: set, Jobs: jb, FFprobePath: "ffprobe"},
		subtitles: subs,
		transcode: &playback.JobHandler{Files: files, Settings: set, Jobs: jb, DataDir: env.cfg.DataDir, FFmpegPath: "ffmpeg"},
		downloads: &downloads.Preparer{Store: downloads.NewStore(env.pool), Files: files, Subtitles: subs, Settings: set,
			DataDir: env.cfg.DataDir, FFmpegPath: "ffmpeg"},
	}
}

// ingest registers the file and queues its probe, returning the media file id.
func (w *worker) ingest(ctx context.Context, path string) string {
	st, err := os.Stat(path)
	if err != nil {
		w.t.Fatal(err)
	}
	lib, err := w.files.ManagedLibraryByKind(ctx, "movies")
	if err != nil {
		w.t.Fatal(err)
	}
	rel, _ := filepath.Rel(lib.Path, path)
	id, err := w.files.UpsertMediaFileStub(ctx, lib.ID, rel, st.Size(), st.ModTime())
	if err != nil {
		w.t.Fatal(err)
	}
	if _, err := w.jobs.EnqueueJob(ctx, "probe", library.ProbePayload{MediaFileID: id}, jobs.EnqueueOpts{}); err != nil {
		w.t.Fatal(err)
	}
	return id
}

// drain runs queued jobs until none is left.
func (w *worker) drain(ctx context.Context) {
	handlers := map[string]jobs.Handler{
		"probe":             w.prober.Handle,
		"extract_subtitles": w.subtitles.HandleExtract,
		"transcode_hls":     w.transcode.Handle,
		downloads.JobType:   w.downloads.Handle,
	}
	types := []string{"probe", "extract_subtitles", "transcode_hls", downloads.JobType}
	for {
		job, err := w.jobs.ClaimJob(ctx, types)
		if errors.Is(err, db.ErrNotFound) {
			return
		}
		if err != nil {
			w.t.Fatal(err)
		}
		start := time.Now()
		w.t.Logf("job %s %s", job.Type, job.Payload)
		jobCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		err = handlers[job.Type](jobCtx, job, func(int) {})
		cancel()
		if err != nil {
			w.t.Fatalf("job %s %s: %v", job.Type, job.Payload, err)
		}
		if err := w.jobs.CompleteJob(ctx, job.ID); err != nil {
			w.t.Fatal(err)
		}
		w.t.Logf("took %s", time.Since(start).Round(time.Millisecond))
	}
}

func (w *worker) titleID(ctx context.Context, title string) string {
	var id string
	name, _, _ := strings.Cut(title, " (")
	if err := w.env.pool.QueryRow(ctx, `SELECT id FROM titles WHERE name = $1`, name).Scan(&id); err != nil {
		w.t.Fatalf("title %s: %v", title, err)
	}
	return id
}

func (w *worker) setTranscode(ctx context.Context, value string) {
	if err := w.settings.Set(ctx, "transcode", json.RawMessage(value)); err != nil {
		w.t.Fatal(err)
	}
}

// checkHLS validates a presentation the payload handed out and plays it in
// AVFoundation when asked to.
func (e *testEnv) checkHLS(t *testing.T, name, path string, s sample, renditions bool) {
	t.Helper()
	target := e.srv.URL + path
	report, err := hls.Validate(context.Background(), hls.HTTPFetcher{Client: e.srv.Client()}, target, hls.Options{})
	if err != nil {
		t.Errorf("%s: %v", name, err)
		return
	}
	for _, issue := range report.Issues {
		if issue.Severity == hls.SeverityError {
			t.Errorf("%s: %s", name, issue)
		} else {
			t.Logf("%s: %s", name, issue)
		}
	}
	t.Logf("%s: %s validated: %d playlists, %d segments", name, path, report.Playlists, report.Segments)
	flags := []string{"--duration", strconv.Itoa(sampleSeconds), "--iframes"}
	if renditions {
		flags = append(flags, "--audio", strconv.Itoa(s.audio), "--subtitles", strconv.Itoa(s.subtitles))
	}
	if strings.Contains(path, "/jit/") {
		flags = flags[:2]
	}
	e.checkAVPlayer(t, name, path, flags)
	e.checkAppleTools(t, name, target)
}

func (e *testEnv) checkAVPlayer(t *testing.T, name, path string, flags []string) {
	t.Helper()
	if os.Getenv("COUCHVERSE_AVPLAYER") != "1" {
		return
	}
	args := append([]string{"../../../scripts/avplayer-probe.swift", e.srv.URL + path, "--play", "3"}, flags...)
	out, err := exec.Command("swift", args...).CombinedOutput()
	if err != nil {
		t.Errorf("%s: AVFoundation: %v\n%s", name, err, out)
		return
	}
	t.Logf("%s: AVFoundation:\n%s", name, out)
}

func (e *testEnv) checkAppleTools(t *testing.T, name, target string) {
	t.Helper()
	if os.Getenv("COUCHVERSE_APPLE_HLS_TOOLS") != "1" {
		return
	}
	if _, err := exec.LookPath("mediastreamvalidator"); err != nil {
		t.Logf("%s: mediastreamvalidator is not installed", name)
		return
	}
	u, _ := url.Parse(target)
	out, err := exec.Command("mediastreamvalidator", u.String()).CombinedOutput()
	if err != nil {
		t.Errorf("%s: mediastreamvalidator: %v\n%s", name, err, out)
	}
}
