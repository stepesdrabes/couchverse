package server_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"couchverse/internal/config"
	"couchverse/internal/feature/artwork"
	"couchverse/internal/feature/couchtv"
	"couchverse/internal/feature/jobs"
	"couchverse/internal/feature/playback"
	"couchverse/internal/hls"
	"couchverse/internal/settings"
)

// TestCouchTvRender renders clips in a real headless Chromium (the broadcast bundle from
// clients/broadcast/dist, `make broadcast`): a short ident, a sting and a credits squeeze
// over a generated program's frames and sound, then assembles them into a break, and checks
// every package with the HLS validator and its length to the frame. It needs
// COUCHVERSE_RENDER_TEST=1, ffmpeg and Chromium (CHROMIUM_PATH, else an installed one).
func TestCouchTvRender(t *testing.T) {
	if os.Getenv("COUCHVERSE_RENDER_TEST") != "1" {
		t.Skip("set COUCHVERSE_RENDER_TEST=1 to render with Chromium")
	}
	bundle := "../../../clients/broadcast/dist"
	if _, err := os.Stat(filepath.Join(bundle, "render.html")); err != nil {
		t.Skip("the broadcast bundle is not built (make broadcast)")
	}
	env := newTestEnv(t, func(cfg *config.Config) { cfg.FFmpegPath, cfg.FFprobePath = "ffmpeg", "ffprobe" })
	ctx := context.Background()
	w := newWorker(t, env)

	// a program to squeeze: 1080p25 with a tone, ingested and probed
	movie := filepath.Join(env.cfg.DataDir, "media", "movies", "Credits Roll (2026)", "Credits Roll (2026).mp4")
	if err := os.MkdirAll(filepath.Dir(movie), 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("ffmpeg", "-y", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "testsrc2=size=1920x1080:rate=25:duration=20",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=20:sample_rate=48000",
		"-c:v", "libx264", "-preset", "ultrafast", "-c:a", "aac", "-shortest", movie).CombinedOutput(); err != nil {
		t.Fatalf("generate: %v %s", err, out)
	}
	fileID := w.ingest(ctx, movie)
	w.drain(ctx)

	store := couchtv.NewStore(env.pool)
	r := &couchtv.Renderer{
		Store: store, Jobs: jobs.NewStore(env.pool), Settings: settings.NewStore(env.pool),
		Artwork: &artwork.Service{Store: artwork.NewStore(env.pool), DataDir: env.cfg.DataDir, FFmpegPath: "ffmpeg"},
		DataDir: env.cfg.DataDir, FFmpeg: "ffmpeg", Chromium: os.Getenv("CHROMIUM_PATH"),
		Bundle: os.DirFS(bundle), RestartEvery: 2,
	}
	if err := r.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	title := couchtv.SpecTitle{Name: "Credits Roll", Genres: []string{"Drama"}, Accents: couchtv.Accents{Backdrop: "#2f6fd0"}}
	creditsAt := 5 * time.Second
	clips := []struct {
		kind   string
		frames int
		spec   couchtv.Spec
	}{
		{couchtv.ClipSqueeze, 265, couchtv.Spec{Template: "squeeze", Ending: &title, Handover: "cursor",
			Program: &couchtv.ProgramSource{Kind: "frames", Frames: couchtv.FramesURLs(fileID, creditsAt, 288)},
			Next:    &couchtv.SqueezeNext{Title: title, Time: "20:15"}, Clock: "20:13",
			Bed: &couchtv.Bed{URL: couchtv.AudioURL(fileID, creditsAt, 11*time.Second), In: 0, Out: 11}}},
		{couchtv.ClipIdentShort, 113, couchtv.Spec{Template: "ident", Cut: "short"}},
		{couchtv.ClipSting, 52, couchtv.Spec{Template: "sting", Duration: 2.08}},
	}
	recipe := couchtv.Recipe{Key: "test", Sound: couchtv.BreakSpec{Template: "break", Loudness: -20}}
	for _, c := range clips {
		id, _, err := store.RequestClip(ctx, c.kind, c.kind, c.spec, c.frames, time.Now().Add(time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		payload, _ := json.Marshal(couchtv.RenderPayload{ClipID: id})
		start := time.Now()
		if err := r.Render(ctx, &jobs.Job{Payload: payload}, func(int) {}); err != nil {
			t.Fatal(err)
		}
		took := time.Since(start)
		t.Logf("%s: %d frames in %s, %.2f fps", c.kind, c.frames, took.Round(time.Millisecond), float64(c.frames)/took.Seconds())
		clip, err := store.Clip(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if clip.State != couchtv.ClipReady || clip.Frames != c.frames {
			t.Fatalf("%s: %s, %d frames", c.kind, clip.State, clip.Frames)
		}
		validatePackage(t, filepath.Join(env.cfg.DataDir, clip.Path), c.frames)
		recipe.Clips = append(recipe.Clips, couchtv.RecipeClip{ClipID: id, Kind: c.kind, At: recipe.Frames, Frames: c.frames})
		recipe.Sound.Items = append(recipe.Sound.Items, couchtv.BreakItem{At: float64(recipe.Frames) / couchtv.FPS, Spec: c.spec})
		recipe.Frames += c.frames
	}

	breakID := insertBreak(t, env, recipe)
	payload, _ := json.Marshal(couchtv.AssemblePayload{BreakID: breakID})
	if err := r.Assemble(ctx, &jobs.Job{Payload: payload}, func(int) {}); err != nil {
		t.Fatal(err)
	}
	b, err := store.Break(ctx, breakID)
	if err != nil {
		t.Fatal(err)
	}
	if b.State != couchtv.BreakReady || b.Frames != recipe.Frames {
		t.Fatalf("break %s, %d frames", b.State, b.Frames)
	}
	validatePackage(t, filepath.Join(env.cfg.DataDir, b.Path), recipe.Frames)
	ev, err := store.Evergreen(ctx)
	if err != nil || ev.ID != breakID {
		t.Errorf("the evergreen break: %v %v", ev, err)
	}
}

// insertBreak stores an evergreen break with a hand-made recipe.
func insertBreak(t *testing.T, env *testEnv, r couchtv.Recipe) string {
	t.Helper()
	body, _ := json.Marshal(r)
	var id string
	if err := env.pool.QueryRow(context.Background(), `INSERT INTO couchtv_breaks (kind, recipe, state, duration_ms)
		VALUES ('evergreen', $1, 'composed', $2) RETURNING id`, body, r.Frames*40).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

// validatePackage checks a rendered package: the validator finds no error in its master
// playlist, playback reads it like a media file's package, and the video is frames long.
func validatePackage(t *testing.T, dir string, frames int) {
	t.Helper()
	report, err := hls.Validate(context.Background(), hls.HTTPFetcher{}, "file://"+filepath.Join(dir, "master.m3u8"), hls.Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, issue := range report.Issues {
		t.Logf("%s: %s", filepath.Base(dir), issue)
	}
	if n := len(report.Errors()); n > 0 {
		t.Errorf("%s: %d validator errors", dir, n)
	}
	p, err := playback.ReadPackageDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if p.Source == nil || len(p.Stereo) != 1 {
		t.Fatalf("%s: not read as a package: %+v", dir, p)
	}
	if got := p.Source.Info.Duration * couchtv.FPS; got < float64(frames)-0.01 || got > float64(frames)+0.01 {
		t.Errorf("%s: %.3f frames of video, want %d", dir, got, frames)
	}
	if p.Source.Info.Codec != "avc1.640028" || p.Source.Info.Width != 1920 || p.Source.Info.Height != 1080 {
		t.Errorf("%s: %+v", dir, p.Source.Info)
	}
}
