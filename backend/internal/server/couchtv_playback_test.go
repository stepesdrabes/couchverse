package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"couchverse/internal/config"
	"couchverse/internal/feature/couchtv"
	"couchverse/internal/feature/jobs"
	"couchverse/internal/feature/library"
	"couchverse/internal/feature/playback"
	"couchverse/internal/hls"
	"couchverse/internal/media"
	"couchverse/internal/settings"
)

// liveProgram is a generated program of the channel and what the browser checks
// expect of it: the tone each audio language plays.
type liveProgram struct {
	name    string
	fileID  string
	tones   map[string]int
	credits time.Duration
	onAir   time.Duration
}

// TestCouchTvLive plays Couch TV through the real handlers: generated programs
// readied by couchtv_prepare and the transcode jobs (a copied 720p source with
// subtitles; HDR10 HEVC in Czech and English from its rung; a 1080p source with
// 10.5 s keyframes from its 1080p rung, cut at its credits), breaks assembled
// as the render sidecar leaves them, two that are not (the evergreen break cut
// to the gap, the standby card looped after it) and a hole in the schedule (the
// standby card), looped around now.
// Players tune in mid-item: hls.js in Chromium (e2e/live.mjs: pairs 20 s apart,
// on hls.js's live edge and holding the program date) and, on macOS,
// AVFoundation, while the HLS validator checks the presentation around four
// boundaries. It needs COUCHVERSE_COUCHTV_LIVE=1, ffmpeg (with libx265) and
// clients/web's dependencies; COUCHVERSE_SPIKE_OUT keeps snapshots and samples,
// COUCHVERSE_SPIKE_SECONDS sets how long the players play, COUCHVERSE_SPIKE_ADDR
// and COUCHVERSE_SPIKE_HOLD serve /watch.html for a while afterwards (a TV or a
// browser by hand), COUCHVERSE_AVPLAYER=0 skips AVFoundation and VIEWERS picks
// the browser viewers (webkit too).
func TestCouchTvLive(t *testing.T) {
	if os.Getenv("COUCHVERSE_COUCHTV_LIVE") != "1" {
		t.Skip("set COUCHVERSE_COUCHTV_LIVE=1 to play the channel")
	}
	for _, tool := range []string{"ffmpeg", "ffprobe", "node"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not installed", tool)
		}
	}
	env := newTestEnv(t, func(cfg *config.Config) { cfg.FFmpegPath, cfg.FFprobePath = "ffmpeg", "ffprobe" })
	ctx := context.Background()
	w := newWorker(t, env)
	setFeatures(t, env, true)
	movies := filepath.Join(env.cfg.DataDir, "media", "movies")
	srt := func(lang, text string) string {
		path := filepath.Join(t.TempDir(), lang+".srt")
		body := fmt.Sprintf("1\n00:00:01,000 --> 00:00:04,000\n%s\n\n2\n00:00:05,500 --> 00:00:07,500\n%s across a segment boundary\n\n"+
			"3\n00:00:20,000 --> 00:00:23,000\n%s, later\n", text, text, text)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	clip := func(title, file string, args ...string) string {
		path := filepath.Join(movies, title, file)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		args = append([]string{"-y", "-hide_banner", "-loglevel", "error"}, append(args, path)...)
		if out, err := exec.Command("ffmpeg", args...).CombinedOutput(); err != nil {
			t.Fatalf("generate %s: %v %s", file, err, out)
		}
		return w.ingest(ctx, path)
	}
	sine := func(hz int, seconds float64) []string {
		return []string{"-f", "lavfi", "-i", fmt.Sprintf("sine=frequency=%d:duration=%g:sample_rate=48000", hz, seconds)}
	}
	video := func(size string, rate int, seconds float64) []string {
		return []string{"-f", "lavfi", "-i", fmt.Sprintf("testsrc2=size=%s:rate=%d:duration=%g", size, rate, seconds)}
	}
	cat := func(parts ...[]string) []string {
		var out []string
		for _, p := range parts {
			out = append(out, p...)
		}
		return out
	}
	harbor := clip("Glass Harbor (2025)", "Glass Harbor (2025).mkv", cat(video("1280x720", 24, 40), sine(330, 40),
		[]string{"-i", srt("en", "Hello from Glass Harbor"), "-i", srt("cs", "Ahoj z Glass Harbor"),
			"-map", "0:v", "-map", "1:a", "-map", "2", "-map", "3", "-c:v", "libx264", "-preset", "veryfast", "-g", "48",
			"-c:a", "aac", "-c:s", "srt", "-metadata:s:a:0", "language=eng", "-metadata:s:s:0", "language=eng",
			"-metadata:s:s:1", "language=ces"})...)
	bloom := clip("Static Bloom (2024)", "Static Bloom (2024).mkv", cat(video("1280x720", 24, 40), sine(330, 40), sine(440, 40),
		[]string{"-i", srt("en", "Static Bloom subtitle"),
			"-map", "0:v", "-map", "1:a", "-map", "2:a", "-map", "3",
			"-vf", "format=yuv420p10le,setparams=color_primaries=bt2020:color_trc=smpte2084:colorspace=bt2020nc",
			"-c:v", "libx265", "-preset", "ultrafast", "-tag:v", "hvc1",
			"-x265-params", "keyint=48:min-keyint=48:hdr10=1:max-cll=1000,400:log-level=error",
			"-c:a", "aac", "-ac", "2", "-c:s", "srt", "-metadata:s:a:0", "language=eng", "-disposition:a:0", "default",
			"-metadata:s:a:1", "language=ces", "-disposition:a:1", "0", "-metadata:s:s:0", "language=eng"})...)
	night := clip("Night Shift (2026)", "Night Shift (2026).mkv", cat(video("1920x1080", 30, 40), sine(550, 40),
		[]string{"-map", "0:v", "-map", "1:a", "-c:v", "libx264", "-preset", "veryfast",
			"-x264-params", "keyint=315:min-keyint=315:scenecut=0", "-c:a", "aac"})...)
	// breaks as couchtv_assemble writes them: 1080p25 H.264 High, 2 s GOP, AAC stereo
	breakClip := func(file string, seconds float64) string {
		return clip("Breaks (2026)", file, cat([]string{"-f", "lavfi", "-i", fmt.Sprintf("color=c=0x1040c0:size=1920x1080:rate=25:duration=%g", seconds)},
			sine(880, seconds), []string{"-map", "0:v", "-map", "1:a",
				"-vf", "drawbox=x='mod(t*240,1700)':y=80:w=160:h=160:color=white:t=fill",
				"-c:v", "libx264", "-preset", "veryfast", "-profile:v", "high", "-crf", "18", "-g", "50", "-keyint_min", "50",
				"-sc_threshold", "0", "-pix_fmt", "yuv420p", "-c:a", "aac", "-b:a", "192k", "-ac", "2"})...)
	}
	break1, break2 := breakClip("break1.mp4", 17.36), breakClip("break2.mp4", 12.48)
	w.drain(ctx)
	for _, id := range []string{break1, break2} {
		if _, err := w.jobs.EnqueueJob(ctx, "transcode_hls", playback.Payload{MediaFileID: id, Variant: media.VariantPackage}, jobs.EnqueueOpts{}); err != nil {
			t.Fatal(err)
		}
	}
	w.drain(ctx)

	// couchtv_prepare readies the programs it finds airing within 48 h
	programs := []*liveProgram{
		{name: "Glass Harbor", fileID: harbor, tones: map[string]int{"cs": 330, "en": 330}},
		{name: "Static Bloom", fileID: bloom, tones: map[string]int{"cs": 440, "en": 330}},
		{name: "Night Shift", fileID: night, tones: map[string]int{"cs": 550, "en": 550}, credits: 30 * time.Second},
	}
	store := couchtv.NewStore(env.pool)
	preparer := &couchtv.Preparer{Store: store, Files: library.NewStore(env.pool), Jobs: w.jobs, Settings: settings.NewStore(env.pool),
		DataDir: env.cfg.DataDir}
	for i, p := range programs {
		at := time.Now().Add(time.Duration(i+1) * time.Hour)
		if _, err := env.pool.Exec(ctx, `INSERT INTO couchtv_airings (starts_at, ends_at, kind, title_id, media_file_id, fill, credits_at_ms)
			SELECT $1, $2, 'program', title_id, id, 'movie', NULLIF($3, 0) FROM media_files WHERE id = $4`,
			at, at.Add(time.Minute), p.credits.Milliseconds(), p.fileID); err != nil {
			t.Fatal(err)
		}
	}
	for range 2 {
		if err := preparer.Handle(ctx, &jobs.Job{}, func(int) {}); err != nil {
			t.Fatal(err)
		}
		w.drain(ctx)
	}
	for _, p := range programs {
		var video string
		var ms int64
		if err := env.pool.QueryRow(ctx, `SELECT video, on_air_ms FROM couchtv_media WHERE media_file_id = $1`, p.fileID).Scan(&video, &ms); err != nil {
			t.Fatalf("%s is not ready: %v", p.name, err)
		}
		p.onAir = time.Duration(ms) * time.Millisecond
		t.Logf("%s airs %s from %s", p.name, p.onAir, video)
	}
	if _, err := env.pool.Exec(ctx, `DELETE FROM couchtv_airings`); err != nil {
		t.Fatal(err)
	}

	// the loop: Glass Harbor, its break; Static Bloom, a 20 s break that is not
	// ready (the evergreen break, the standby card after it); Night Shift, an 8 s
	// break that is not ready either (the evergreen break cut short); a 5 s hole
	// (the standby card). The card is 4 s, so both loop it.
	breakDir := func(id string) string { return filepath.Join("cache", "hls", id) }
	lengthOf := func(id string) time.Duration {
		p, err := playback.ReadPackageDir(filepath.Join(env.cfg.DataDir, breakDir(id)))
		if err != nil || p.Source == nil {
			t.Fatalf("break package %s: %v", id, err)
		}
		return time.Duration(p.Source.Info.Duration * float64(time.Second)).Truncate(time.Millisecond)
	}
	insert := func(kind string, start, end time.Time, p *liveProgram) string {
		var id string
		var file any
		var credits int64
		if p != nil {
			file, credits = p.fileID, p.credits.Milliseconds()
		}
		if err := env.pool.QueryRow(ctx, `INSERT INTO couchtv_airings (starts_at, ends_at, kind, title_id, media_file_id, fill, credits_at_ms)
			VALUES ($1, $2, $3, (SELECT title_id FROM media_files WHERE id = $4::uuid), $4::uuid, CASE WHEN $3 = 'program' THEN 'movie' END,
			        NULLIF($5, 0)) RETURNING id`, start, end, kind, file, credits).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	// the evergreen break, as the composer and the sidecar leave it
	if _, err := env.pool.Exec(ctx, `INSERT INTO couchtv_breaks (kind, recipe, state, path, duration_ms) VALUES ('evergreen', '{}', 'ready', $1, $2)`,
		breakDir(break2), lengthOf(break2).Milliseconds()); err != nil {
		t.Fatal(err)
	}
	standbyCard(t, env, 4, 660)
	start := time.Now().Add(-95 * time.Second).Truncate(time.Second)
	hold, _ := time.ParseDuration(os.Getenv("COUCHVERSE_SPIKE_HOLD"))
	for at := start; at.Before(time.Now().Add(30*time.Minute + hold)); {
		for i, p := range programs {
			end := at.Add(p.onAir)
			insert("program", at, end, p)
			gap := []time.Duration{lengthOf(break1), 20 * time.Second, 8 * time.Second}[i]
			id := insert("break", end, end.Add(gap), nil)
			if i == 0 {
				if _, err := env.pool.Exec(ctx, `INSERT INTO couchtv_breaks (airing_id, recipe, state, path) VALUES ($1, '{}', 'ready', $2)`,
					id, breakDir(break1)); err != nil {
					t.Fatal(err)
				}
			}
			at = end.Add(gap)
		}
		at = at.Add(5 * time.Second)
	}

	status, body, _ := env.request(t, env.clients[memberName], "GET", "/couchtv/tune?lang=cs", nil)
	if status != http.StatusOK {
		t.Fatalf("tune: %d %s", status, body)
	}
	var tune struct {
		StreamURL      string `json:"streamUrl"`
		TargetDuration int    `json:"targetDuration"`
	}
	if err := json.Unmarshal(body, &tune); err != nil {
		t.Fatal(err)
	}
	masterPath, _, _ := strings.Cut(tune.StreamURL, "?")

	// the schedule as the browser checks read it: every part of every airing
	live := couchtv.NewChannel(store, couchtv.NewDiskMedia(env.cfg.DataDir, couchtv.NewSlate(env.cfg.DataDir, "ffmpeg")))
	parts, err := live.Lineup(ctx, start, time.Now().Add(30*time.Minute+hold))
	if err != nil {
		t.Fatal(err)
	}
	describe := describeLineup(t, env, parts, programs, tune.TargetDuration)

	mux := http.NewServeMux()
	mux.Handle("/api/", env.srv.Config.Handler)
	mux.HandleFunc("/schedule.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(describe)
	})
	mux.HandleFunc("/clock", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"now":%d}`, time.Now().UnixMilli())
	})
	mux.HandleFunc("/watch.html", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		io.WriteString(w, watchPage(tune.TargetDuration, masterPath))
	})
	mux.HandleFunc("/hls.min.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		http.ServeFile(w, r, "../../../clients/web/node_modules/hls.js/dist/hls.min.js")
	})
	srv := httptest.NewUnstartedServer(mux)
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
	master := srv.URL + masterPath
	t.Logf("channel on %s, a player on %s/watch.html", master, srv.URL)

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
	players, stop := context.WithCancel(ctx)
	defer stop()
	browsers := exec.CommandContext(players, "node", "e2e/live.mjs")
	browsers.Dir = "../../../clients/web"
	browsers.Env = append(os.Environ(), "SERVER="+srv.URL, "SECONDS="+seconds, "MASTER="+masterPath)
	if os.Getenv("VIEWERS") == "" {
		// hls.js in WebKit misses the first subtitle segment of a program after
		// the slate (docs/couch-tv-plan.md, Phase 4); VIEWERS=...,webkit plays it too
		browsers.Env = append(browsers.Env, "VIEWERS=chromium,chromium +20s,chromium pdt,chromium pdt +20s")
	}
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
	var boundaries []time.Time
	for _, p := range parts {
		if p.Start.After(time.Now().Add(3*time.Second)) && len(boundaries) < 4 {
			boundaries = append(boundaries, p.Start)
		}
	}
	validateLive(t, srv.Client(), master, boundaries, os.Getenv("COUCHVERSE_SPIKE_OUT"))
	wg.Wait()
	for _, name := range []string{"hls.js", "AVFoundation"} {
		if out, ok := results[name]; ok {
			t.Logf("%s:\n%s", name, out)
		}
	}
	if out, ok := results["AVFoundation"]; ok {
		var starts []time.Time
		for _, p := range parts {
			starts = append(starts, p.Start)
		}
		checkAVPlayerLive(t, out, starts)
	}
	if hold > 0 {
		t.Logf("serving %s/watch.html for %s", srv.URL, hold)
		time.Sleep(hold)
	}
}

// describeLineup is the channel's parts as e2e/live.mjs reads them (Unix ms):
// what each is, the tone each language plays and the cues it shows.
func describeLineup(t *testing.T, env *testEnv, parts []couchtv.LineupPart, programs []*liveProgram, target int) []byte {
	t.Helper()
	ctx := context.Background()
	cues := map[string]map[string][]spikeCue{} // file -> lang -> cues from the file's start
	for _, p := range programs {
		cues[p.fileID] = map[string][]spikeCue{}
		rows, err := env.pool.Query(ctx, `SELECT lang, path FROM subtitles WHERE media_file_id = $1`, p.fileID)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var lang, path string
			if err := rows.Scan(&lang, &path); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(env.cfg.DataDir, path))
			if err != nil {
				t.Fatal(err)
			}
			vtt, err := hls.ParseWebVTT(string(data))
			if err != nil {
				t.Fatal(err)
			}
			for _, c := range vtt.Cues {
				cues[p.fileID][media.BCP47(lang)] = append(cues[p.fileID][media.BCP47(lang)],
					spikeCue{Start: int64(c.Start * 1000), End: int64(c.End * 1000), Text: c.Payload})
			}
		}
		rows.Close()
	}
	var items []spikeScheduleItem
	for _, part := range parts {
		item := spikeScheduleItem{Name: part.Kind, Kind: part.Kind, Start: part.Start.UnixMilli(), End: part.End.UnixMilli(),
			Tones: map[string]int{"cs": 0, "en": 0}, Subtitles: map[string][]spikeCue{}, Video: map[string]any{}}
		switch part.Kind {
		case couchtv.PartBreak:
			item.Tones = map[string]int{"cs": 880, "en": 880}
		case couchtv.PartStandby:
			item.Tones = map[string]int{"cs": 660, "en": 660}
		case couchtv.PartProgram:
			for _, p := range programs {
				if p.fileID != part.File {
					continue
				}
				item.Name, item.Tones = p.name, p.tones
				for lang, list := range cues[p.fileID] {
					for _, c := range list {
						if c.Start < part.End.Sub(part.Start).Milliseconds() {
							item.Subtitles[lang] = append(item.Subtitles[lang],
								spikeCue{Start: item.Start + c.Start, End: item.Start + c.End, Text: c.Text})
						}
					}
				}
			}
		}
		items = append(items, item)
	}
	t.Logf("lineup: %d parts from %s", len(items), parts[0].Start.Format(time.TimeOnly))
	for _, it := range items[:min(len(items), 8)] {
		t.Logf("  %s %s +%.3fs", time.UnixMilli(it.Start).Format("15:04:05.000"), it.Name, float64(it.End-it.Start)/1000)
	}
	data, err := json.Marshal(map[string]any{"items": items, "targetDuration": target})
	if err != nil {
		t.Fatal(err)
	}
	return data
}
