package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"couchverse/internal/config"
	"couchverse/internal/db"
	"couchverse/internal/feature/couchtv"
	"couchverse/internal/feature/jobs"
	"couchverse/internal/feature/library"
)

// analysisWorker runs the analysis jobs the way the app wires them.
type analysisWorker struct {
	*worker
	markers *library.MarkerAnalyzer
	couchtv *couchtv.Analyzer
}

func newAnalysisWorker(t *testing.T, env *testEnv) *analysisWorker {
	w := newWorker(t, env)
	markers := &library.MarkerAnalyzer{Files: w.files, Jobs: w.jobs, DataDir: env.cfg.DataDir, FFmpegPath: "ffmpeg"}
	tv := &couchtv.Analyzer{Store: couchtv.NewStore(env.pool), Jobs: w.jobs, Markers: markers, FFmpegPath: "ffmpeg"}
	markers.Analyzed = tv.Queue
	return &analysisWorker{worker: w, markers: markers, couchtv: tv}
}

// run claims and runs the runnable jobs of the given types until none is left.
func (w *analysisWorker) run(ctx context.Context, types ...string) {
	handlers := map[string]jobs.Handler{"probe": w.prober.Handle, library.MarkersJob: w.markers.Handle, couchtv.AnalyzeJob: w.couchtv.Handle}
	for {
		job, err := w.jobs.ClaimJob(ctx, types)
		if errors.Is(err, db.ErrNotFound) {
			return
		}
		if err != nil {
			w.t.Fatal(err)
		}
		if err := handlers[job.Type](ctx, job, func(int) {}); err != nil {
			w.t.Fatalf("job %s %s: %v", job.Type, job.Payload, err)
		}
		if err := w.jobs.CompleteJob(ctx, job.ID); err != nil {
			w.t.Fatal(err)
		}
	}
}

func (w *analysisWorker) markersOf(ctx context.Context, fileID string) map[string]library.MediaMarker {
	m, err := w.files.MediaMarkers(ctx, fileID)
	if err != nil {
		w.t.Fatal(err)
	}
	out := map[string]library.MediaMarker{}
	for _, mk := range m.Markers {
		out[mk.Kind+"/"+mk.Source] = mk
	}
	return out
}

func near(ms *int, seconds, within float64) bool {
	return ms != nil && math.Abs(float64(*ms)/1000-seconds) <= within
}

// creditsRoll is a tall picture of lines of white "text" on black that the clip scrolls
// through as an end crawl.
func creditsRoll(t *testing.T, path string) {
	img := image.NewGray(image.Rect(0, 0, 320, 2400))
	for i := range img.Pix {
		img.Pix[i] = 16
	}
	for line := 0; line < 2400/16; line++ {
		width := 80 + line*37%140
		for y := line*16 + 6; y < line*16+9; y++ {
			for x := 160 - width/2; x < 160+width/2; x++ {
				if x%9 < 6 {
					img.SetGray(x, y, color.Gray{Y: 235})
				}
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestMediaAnalysis analyzes a generated movie end to end (its credits from the crawl
// and the music, the channel's loudness, promo audio and shots, the waveform) and a
// season of a series from fingerprints taken in production, then checks what the
// channel's schedule and the admin operations make of the markers.
func TestMediaAnalysis(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not installed")
	}
	env := newTestEnv(t, func(cfg *config.Config) { cfg.FFmpegPath, cfg.FFprobePath = "ffmpeg", "ffprobe" })
	ctx := context.Background()
	w := newAnalysisWorker(t, env)

	// a movie: 60 s of a moving test pattern under speech-like sound (a tone in one
	// second, out the next), then 90 s of a credits crawl under steady music
	lib, err := w.files.ManagedLibraryByKind(ctx, "movies")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(lib.Path, "Crawl Test (2020)", "Crawl Test (2020).mp4")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	roll := filepath.Join(t.TempDir(), "roll.png")
	creditsRoll(t, roll)
	if out, err := exec.Command("ffmpeg", "-y", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "testsrc2=size=320x180:rate=25:duration=60",
		"-loop", "1", "-framerate", "25", "-t", "90", "-i", roll,
		"-f", "lavfi", "-i", `aevalsrc=exprs=0.3*sin(2*PI*440*t)*if(lt(t\,60)\,lt(mod(t\,2)\,1)\,1):s=48000:d=150`,
		"-filter_complex", "[1:v]crop=320:180:0:'min(t*40,2220)',format=yuv420p,setsar=1[c];[0:v]format=yuv420p,setsar=1[s];[s][c]concat=n=2:v=1:a=0[v]",
		"-map", "[v]", "-map", "2:a", "-c:v", "libx264", "-preset", "veryfast", "-c:a", "aac", path).CombinedOutput(); err != nil {
		t.Fatalf("generate: %v %s", err, out)
	}
	fileID := w.ingest(ctx, path)
	w.run(ctx, "probe", library.MarkersJob)
	movieID := w.titleID(ctx, "Crawl Test (2020)")

	found := w.markersOf(ctx, fileID)
	if c := found["credits/auto"]; !near(c.StartMs, 60, 1) || c.EndMs != nil {
		t.Errorf("credits %+v, want from 60 s", c)
	}
	if m := found["credits_music/auto"]; !near(m.StartMs, 60, 0.5) || !near(m.EndMs, 150, 0.5) {
		t.Errorf("credits music %+v, want 60-150 s", m)
	}

	// the markers queued the channel's analysis of the title (a little later)
	var queued bool
	if err := env.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM jobs WHERE type = $1 AND status = 'pending'
		AND payload->>'titleId' = $2)`, couchtv.AnalyzeJob, movieID).Scan(&queued); err != nil || !queued {
		t.Fatalf("no %s queued for the movie (%v)", couchtv.AnalyzeJob, err)
	}
	if _, err := env.pool.Exec(ctx, `UPDATE jobs SET run_at = now() WHERE type = $1`, couchtv.AnalyzeJob); err != nil {
		t.Fatal(err)
	}
	w.run(ctx, couchtv.AnalyzeJob)

	var title couchtv.CouchTvTitle
	status, body, _ := env.do(t, apiCase{as: "admin", method: "GET", path: "/admin/couchtv/titles/" + movieID})
	if status != 200 || json.Unmarshal(body, &title) != nil {
		t.Fatalf("title: %d %s", status, body)
	}
	if title.LoudnessLufs == nil || *title.LoudnessLufs > -10 || *title.LoudnessLufs < -30 {
		t.Errorf("loudness %v", title.LoudnessLufs)
	}
	if p := title.PromoAudio; p == nil || p.MediaFileID != fileID || p.StartMs != 60_000 || p.DurationMs != 40_000 || p.Source != "auto" {
		t.Errorf("promo audio %+v, want the credits music's first 40 s", title.PromoAudio)
	}
	if len(title.Shots) < 3 || title.ShotsSource != "auto" || title.AnalyzedVersion != couchtv.AnalyzeVersion {
		t.Errorf("shots %+v (%s), version %d", title.Shots, title.ShotsSource, title.AnalyzedVersion)
	}
	for i, s := range title.Shots {
		if s.DurationMs < 1000 || s.DurationMs > 3000 || s.DurationMs%40 != 0 || s.StartMs < 7_500 || s.StartMs > 75_000 ||
			i > 0 && s.StartMs <= title.Shots[i-1].StartMs {
			t.Errorf("shot %d: %+v", i, s)
		}
	}

	var wave couchtv.CouchTvWaveform
	status, body, _ = env.do(t, apiCase{as: "admin", method: "GET",
		path: "/admin/couchtv/media-files/" + fileID + "/waveform?fromMs=50000&toMs=70000&points=40"})
	if status != 200 || json.Unmarshal(body, &wave) != nil {
		t.Fatalf("waveform: %d %s", status, body)
	}
	// the tone comes and goes before 60 s and holds after
	if len(wave.Loudness) != 40 || wave.StepMs != 500 || slicesMin(wave.Loudness[:20]) > -50 || slicesMin(wave.Loudness[22:]) < -20 {
		t.Errorf("waveform %+v", wave)
	}

	// a season of a series, from fingerprints of South Park taken in production: the
	// intro after a cold open (season 1) and the end credits (season 13)
	if _, err := env.pool.Exec(ctx, `INSERT INTO episodes (id, season_id, episode_number, name)
		VALUES ('00000000-0000-4000-8000-000000000304', $1, 3, 'Three')`, season1ID); err != nil {
		t.Fatal(err)
	}
	if _, err := env.pool.Exec(ctx, `
		INSERT INTO media_files (id, library_id, episode_id, path, size_bytes, container, video_codec, audio_codec, duration_seconds,
		                         probe, scanned_at, audio_role)
		VALUES ('00000000-0000-4000-8000-000000000505', 2, '00000000-0000-4000-8000-000000000304', 'Static Bloom S01E03.mkv',
		        1, 'matroska', 'h264', 'aac', 2600, '{}', now(), 'primary')`); err != nil {
		t.Fatal(err)
	}
	episodeFiles := []struct {
		id       string
		duration float64
		fixture  int
	}{{hdrFileID, 2520, 2}, {ladderFileID, 2640, 3}, {"00000000-0000-4000-8000-000000000505", 2600, 4}}
	for _, f := range episodeFiles {
		for part, fixture := range map[string]string{
			"head": fmt.Sprintf("south-park-s01e%02d.head.fp", f.fixture),
			"tail": fmt.Sprintf("south-park-s13e%02d.tail.fp", f.fixture),
		} {
			raw, err := os.ReadFile("../feature/library/testdata/fingerprints/" + fixture)
			if err != nil {
				t.Fatal(err)
			}
			// the analyzer's cache (version 1 of its fingerprints)
			dir := filepath.Join(env.cfg.DataDir, "cache", "fingerprints", f.id)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, part+"-v1.fp"), raw, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := w.jobs.EnqueueJob(ctx, library.MarkersJob, library.AnalyzePayload{TitleID: seriesID, Seasons: []int{1}}, jobs.EnqueueOpts{}); err != nil {
		t.Fatal(err)
	}
	w.run(ctx, library.MarkersJob)
	wantIntro := []float64{106.6, 83.2, 108.7}
	for i, f := range episodeFiles {
		m := w.markersOf(ctx, f.id)
		if intro := m["intro/auto"]; !near(intro.StartMs, wantIntro[i], 0.2) || intro.EndMs == nil {
			t.Errorf("episode %d: intro %+v, want from %.1f s", i+1, intro, wantIntro[i])
		}
		if credits := m["credits/auto"]; !near(credits.StartMs, f.duration-360+330.8, 0.2) {
			t.Errorf("episode %d: credits %+v, want from %.1f s", i+1, credits, f.duration-360+330.8)
		}
	}

	// the series' promo plays an intro; its files are not on disk, so it is measured
	// from the markers alone
	if err := w.couchtv.Handle(ctx, &jobs.Job{Payload: json.RawMessage(`{"titleId":"` + seriesID + `","force":true}`)}, func(int) {}); err != nil {
		t.Fatal(err)
	}
	tv := couchtv.NewStore(env.pool)
	row, err := tv.Title(ctx, seriesID)
	if err != nil {
		t.Fatal(err)
	}
	if p := row.PromoAudio; p == nil || p.DurationMs < 32_000 || p.DurationMs > 32_200 {
		t.Errorf("series promo audio %+v, want the season's median intro", p)
	}

	// programs air to their credits; an admin's "none" overrides the analysis
	status, body, _ = env.do(t, apiCase{as: "admin", method: "PUT", path: "/admin/media-files/" + fileID + "/markers",
		body: json.RawMessage(`{"markers":[{"kind":"credits","none":true}]}`)})
	if status != 200 {
		t.Fatalf("markers: %d %s", status, body)
	}
	if _, err := env.pool.Exec(ctx, `UPDATE titles SET status = 'published' WHERE id = $1`, movieID); err != nil {
		t.Fatal(err)
	}
	channel, err := tv.Library(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range channel.Movies {
		if m.ID == movieID && m.CreditsAt != 0 {
			t.Errorf("the movie airs to %v despite the admin's none", m.CreditsAt)
		}
	}
	for _, s := range channel.Series {
		for _, e := range s.Episodes {
			if e.FileID == ladderFileID && math.Abs(e.CreditsAt.Seconds()-(2640-360+330.8)) > 0.2 {
				t.Errorf("episode 2 airs to %v", e.CreditsAt)
			}
		}
	}
}

func slicesMin(v []float64) float64 {
	out := math.Inf(1)
	for _, x := range v {
		out = min(out, x)
	}
	return out
}
