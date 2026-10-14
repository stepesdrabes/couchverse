package server_test

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"couchverse/internal/config"
	"couchverse/internal/feature/artwork"
	"couchverse/internal/feature/couchtv"
	"couchverse/internal/feature/jobs"
	"couchverse/internal/settings"
)

// TestCouchTvTrailerRender renders a trailer's body in a real headless Chromium from a generated
// movie with two audio tracks, the default one at 440 Hz and the English one at 880: the
// sidecar extracts each scene's frames and its sound from the named stream (the English one,
// stereo FLAC) before the clip loads, and the clip's package is as long as its frames. It needs
// what TestCouchTvRender does: COUCHVERSE_RENDER_TEST=1, ffmpeg, Chromium and the bundle.
func TestCouchTvTrailerRender(t *testing.T) {
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

	movie := filepath.Join(env.cfg.DataDir, "media", "movies", "Two Tracks (2026)", "Two Tracks (2026).mkv")
	if err := os.MkdirAll(filepath.Dir(movie), 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("ffmpeg", "-y", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "testsrc2=size=640x360:rate=25:duration=40",
		"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000:duration=40", "-f", "lavfi", "-i", "sine=frequency=880:sample_rate=48000:duration=40",
		"-map", "0:v", "-map", "1:a", "-map", "2:a", "-c:v", "libx264", "-preset", "ultrafast", "-c:a", "aac",
		"-metadata:s:a:0", "language=ces", "-metadata:s:a:1", "language=eng", "-disposition:a:0", "default", "-disposition:a:1", "0",
		movie).CombinedOutput(); err != nil {
		t.Fatalf("generate: %v %s", err, out)
	}
	fileID := w.ingest(ctx, movie)
	w.drain(ctx)
	var stream int
	if err := env.pool.QueryRow(ctx, `SELECT stream_index FROM audio_streams WHERE media_file_id = $1 AND lang IN ('en', 'eng')`, fileID).Scan(&stream); err != nil {
		t.Fatal(err)
	}

	var scenes []couchtv.SpecScene
	for _, at := range []time.Duration{5 * time.Second, 15 * time.Second, 25 * time.Second} {
		scenes = append(scenes, couchtv.SpecScene{
			Frames: couchtv.FramesURLs(fileID, at, 50),
			Sound:  couchtv.Bed{URL: couchtv.StreamURL(fileID, stream, at, 2*time.Second), Out: 2},
			Lines:  [][2]float64{{0.3, 1.6}},
		})
	}
	title := couchtv.SpecTitle{Name: "Two Tracks", Genres: []string{"Drama"}, Accents: couchtv.Accents{Backdrop: "#2f6fd0"}}
	spec := couchtv.Spec{Template: "trailer", Part: "body", Mood: "cinematic", Title: &title, Scenes: scenes}

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
	id, _, err := store.RequestClip(ctx, couchtv.ClipTrailerBody, "trailer-test", spec, 0, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(couchtv.RenderPayload{ClipID: id})
	start := time.Now()
	if err := r.Render(ctx, &jobs.Job{Payload: payload}, func(int) {}); err != nil {
		t.Fatal(err)
	}
	clip, err := store.Clip(ctx, id)
	if err != nil || clip.State != couchtv.ClipReady || clip.Frames < 150 {
		t.Fatalf("the trailer: %+v %v", clip, err)
	}
	t.Logf("trailer body: %d frames in %s, %.2f fps", clip.Frames, time.Since(start).Round(time.Millisecond), float64(clip.Frames)/time.Since(start).Seconds())
	validatePackage(t, filepath.Join(env.cfg.DataDir, clip.Path), clip.Frames)

	// each scene's frames and its sound, cut from the English stream: 2 s of stereo FLAC at 880 Hz
	for _, at := range []string{"5000", "15000", "25000"} {
		if _, err := os.Stat(filepath.Join(env.cfg.DataDir, "couchtv", "frames", fileID, at+"_50", ".done")); err != nil {
			t.Errorf("scene at %s ms: its frames were not extracted: %v", at, err)
		}
		flac := filepath.Join(env.cfg.DataDir, "couchtv", "audio", fileID, "s"+strconv.Itoa(stream)+"_"+at+"_2000.flac")
		raw, err := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-i", flac, "-f", "s16le", "-ac", "1", "-ar", "48000", "-").Output()
		if err != nil {
			t.Fatalf("scene at %s ms: its sound: %v", at, err)
		}
		samples := make([]float64, len(raw)/2)
		for i := range samples {
			samples[i] = float64(int16(binary.LittleEndian.Uint16(raw[2*i:]))) / 32768
		}
		if len(samples) < 95000 || len(samples) > 97000 {
			t.Errorf("scene at %s ms: %d samples, want 2 s", at, len(samples))
		}
		if english, def := tone(samples, 880), tone(samples, 440); english < 10*def {
			t.Errorf("scene at %s ms: %.3f at 880 Hz and %.3f at 440 Hz: not the English stream's sound", at, english, def)
		}
	}
}

// tone is the amplitude of one frequency in 48 kHz samples (Goertzel).
func tone(x []float64, freq float64) float64 {
	k := 2 * math.Cos(2*math.Pi*freq/48000)
	var s1, s2 float64
	for _, v := range x {
		s1, s2 = v+k*s1-s2, s1
	}
	return math.Sqrt(s1*s1+s2*s2-k*s1*s2) * 2 / float64(len(x))
}
