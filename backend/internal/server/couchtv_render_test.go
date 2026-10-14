package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
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
// over a generated program's frames (composited by ffmpeg) and sound, then assembles them
// into a break, and checks every package with the HLS validator and its length to the
// frame. It needs COUCHVERSE_RENDER_TEST=1, ffmpeg and Chromium (CHROMIUM_PATH, else an
// installed one).
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
		{couchtv.ClipSqueeze, 265, couchtv.Spec{Template: "squeeze", Ending: &title, Handover: "ident",
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
		if c.kind == couchtv.ClipSqueeze {
			checkComposite(t, filepath.Join(env.cfg.DataDir, clip.Path), movie, creditsAt)
		}
		recipe.Clips = append(recipe.Clips, couchtv.RecipeClip{ClipID: id, Kind: c.kind, At: recipe.Frames, Frames: c.frames})
		recipe.Sound.Items = append(recipe.Sound.Items, couchtv.BreakItem{At: float64(recipe.Frames) / couchtv.FPS, Spec: c.spec})
		recipe.Frames += c.frames
	}

	// every clip kept its sound as a break item; one kept before clips did (or pruned since)
	// is rendered again when the break assembles
	sounds, _ := filepath.Glob(filepath.Join(env.cfg.DataDir, "couchtv", "sounds", "*.wav"))
	if len(sounds) != len(clips) {
		t.Fatalf("%d item sounds kept for %d clips", len(sounds), len(clips))
	}
	if err := os.Remove(sounds[0]); err != nil {
		t.Fatal(err)
	}
	breakID := insertBreak(t, env, recipe)
	payload, _ := json.Marshal(couchtv.AssemblePayload{BreakID: breakID})
	start := time.Now()
	if err := r.Assemble(ctx, &jobs.Job{Payload: payload}, func(int) {}); err != nil {
		t.Fatal(err)
	}
	t.Logf("assembled in %s, one item's sound rendered again", time.Since(start).Round(time.Millisecond))
	if _, err := os.Stat(sounds[0]); err != nil {
		t.Errorf("the missing item's sound was not kept: %v", err)
	}
	b, err := store.Break(ctx, breakID)
	if err != nil {
		t.Fatal(err)
	}
	if b.State != couchtv.BreakReady || b.Frames != recipe.Frames {
		t.Fatalf("break %s, %d frames", b.State, b.Frames)
	}
	validatePackage(t, filepath.Join(env.cfg.DataDir, b.Path), recipe.Frames)
	if lufs := measureLoudness(t, filepath.Join(env.cfg.DataDir, b.Path, "audio-1-aac", "index.m3u8")); lufs < -20.5 || lufs > -19.5 {
		t.Errorf("the break's sound is %.1f LUFS, the recipe asked for -20", lufs)
	}
	ev, err := store.Evergreen(ctx)
	if err != nil || ev.ID != breakID {
		t.Errorf("the evergreen break: %v %v", ev, err)
	}

	// the same clips around a slot for the news (the sting holds it): assembled in sections,
	// a package each, the slot's sound at its own loudness
	slotted := couchtv.Recipe{Key: "slotted", Sound: couchtv.BreakSpec{Template: "break", Loudness: -20}}
	for _, i := range []int{0, 2, 1} {
		c := recipe.Clips[i]
		c.At = slotted.Frames
		slotted.Clips = append(slotted.Clips, c)
		slotted.Sound.Items = append(slotted.Sound.Items, couchtv.BreakItem{At: float64(c.At) / couchtv.FPS, Spec: clips[i].spec})
		slotted.Frames += c.Frames
	}
	slotted.Slots = []couchtv.RecipeSlot{{Kind: couchtv.BulletinNews, At: 265, Frames: 52, Loudness: -23}}
	breakID = insertBreak(t, env, slotted)
	payload, _ = json.Marshal(couchtv.AssemblePayload{BreakID: breakID})
	if err := r.Assemble(ctx, &jobs.Job{Payload: payload}, func(int) {}); err != nil {
		t.Fatal(err)
	}
	if b, err = store.Break(ctx, breakID); err != nil || b.State != couchtv.BreakReady {
		t.Fatalf("the break with a slot: %+v %v", b, err)
	}
	for i, frames := range []int{265, 52, 113} {
		validatePackage(t, filepath.Join(env.cfg.DataDir, b.Path, fmt.Sprint(i)), frames)
	}
	if lufs := measureLoudness(t, filepath.Join(env.cfg.DataDir, b.Path, "1", "audio-1-aac", "index.m3u8")); lufs < -23.6 || lufs > -22.4 {
		t.Errorf("the slot's sound is %.1f LUFS, the recipe asked for -23", lufs)
	}

	// the bulletins, as couchtv_bulletin_render renders them: a rendition at every slot loudness
	day := func(label, weekday string) couchtv.SpecWeatherDay {
		return couchtv.SpecWeatherDay{Label: label, Weekday: weekday, Short: weekday[:3], Min: 4, Max: 15, Code: 2}
	}
	days := []couchtv.SpecWeatherDay{day("Today", "Tuesday"), day("Tomorrow", "Wednesday"), day("Thursday", "Thursday"),
		day("Friday", "Friday"), day("Saturday", "Saturday")}
	quiet := []couchtv.SpecWarning{}
	bulletins := []struct {
		kind   string
		frames int
		spec   couchtv.Spec
	}{
		{couchtv.ClipWeather, 375, couchtv.Spec{Template: "weather", Duration: 15, AsOf: "17:00", Warnings: &quiet,
			Cities: []couchtv.SpecCity{{Name: "Prague", Lat: 50.088, Lon: 14.4208, Now: couchtv.SpecWeatherNow{Temperature: 15, Code: 1, Day: true}, Days: days},
				{Name: "Ostrava", Lat: 49.8347, Lon: 18.282, Now: couchtv.SpecWeatherNow{Temperature: -2, Code: 71}, Days: days}},
			Credits: &couchtv.SpecCredits{Forecast: couchtv.WeatherAttribution, Warnings: couchtv.WarningsAttribution, Borders: couchtv.OutlineAttribution}}},
		{couchtv.ClipNews, 500, couchtv.Spec{Template: "news", Duration: 20, AsOf: "17:30",
			Stories: []couchtv.SpecStory{{Headline: "Bulharsko hlásí další útok na lodě v Černém moři, jedna se potopila", Outlet: "ČT24", Outlets: 4},
				{Headline: "Letadlo do Norimberku doprovázely stíhačky a policie uzavřela letiště. Piloti omylem nahlásili nouzi",
					Outlet: "iROZHLAS", Outlets: 5},
				{Headline: "Spojenci nás nevarovali před posledním útokem na Kyjev, uvedl Zelenskyj", Outlet: "ČT24", Outlets: 3}},
			Credit: "Headlines from the outlets’ RSS feeds, as of 17:30"}},
	}
	for _, bl := range bulletins {
		id, _, err := store.RequestClip(ctx, bl.kind, bl.kind+"-test", bl.spec, bl.frames, time.Now().Add(time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		payload, _ := json.Marshal(couchtv.RenderPayload{ClipID: id})
		start := time.Now()
		if err := r.Render(ctx, &jobs.Job{Payload: payload}, func(int) {}); err != nil {
			t.Fatal(err)
		}
		took := time.Since(start)
		t.Logf("%s: %d frames in %s, %.2f fps", bl.kind, bl.frames, took.Round(time.Millisecond), float64(bl.frames)/took.Seconds())
		clip, err := store.Clip(ctx, id)
		if err != nil || clip.State != couchtv.ClipReady || clip.Frames != bl.frames {
			t.Fatalf("%s: %+v %v", bl.kind, clip, err)
		}
		dir := filepath.Join(env.cfg.DataDir, clip.Path)
		validateRenditions(t, dir, bl.frames, 9)
		for _, lufs := range []int{-23, -19, -16} {
			got, peak := measureLoudnessPeak(t, filepath.Join(dir, fmt.Sprintf("audio-l%d-aac", -lufs), "index.m3u8"))
			// quieter than asked only where its true peak reached the ceiling: never clipped
			capped := got < float64(lufs) && peak > -1.6
			if math.Abs(got-float64(lufs)) > 0.6 && !capped {
				t.Errorf("%s at %d LUFS measures %.1f (true peak %.1f dBTP)", bl.kind, lufs, got, peak)
			}
		}
	}
}

// measureLoudness is the integrated loudness ffmpeg's ebur128 measures of a rendition.
func measureLoudness(t *testing.T, playlist string) float64 {
	t.Helper()
	lufs, _ := measureLoudnessPeak(t, playlist)
	return lufs
}

// measureLoudnessPeak is the integrated loudness and the true peak (dBTP) ffmpeg's ebur128
// measures of a rendition.
func measureLoudnessPeak(t *testing.T, playlist string) (lufs, peak float64) {
	t.Helper()
	out, err := exec.Command("ffmpeg", "-hide_banner", "-nostats", "-i", playlist, "-af", "ebur128=peak=true", "-f", "null", "-").CombinedOutput()
	if err != nil {
		t.Fatalf("ebur128: %v %s", err, out)
	}
	m := regexp.MustCompile(`I:\s+(-?[\d.]+) LUFS`).FindAllSubmatch(out, -1)
	p := regexp.MustCompile(`Peak:\s+(-?[\d.]+) dBFS`).FindAllSubmatch(out, -1)
	if m == nil || p == nil {
		t.Fatalf("no loudness or peak in %s", out)
	}
	lufs, _ = strconv.ParseFloat(string(m[len(m)-1][1]), 64)
	peak, _ = strconv.ParseFloat(string(p[len(p)-1][1]), 64)
	return lufs, peak
}

// checkComposite compares the squeeze's panel during its hold (6 s in: 1008x567 at 96,236,
// inside its rounded corners) with the program at that moment: ffmpeg laid the program
// there, and the page drew no frames.
func checkComposite(t *testing.T, dir, movie string, from time.Duration) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(filepath.Dir(filepath.Dir(dir)), "frames")); !os.IsNotExist(err) {
		t.Errorf("the squeeze's program was extracted as frames (%v)", err)
	}
	out, err := exec.Command("ffmpeg", "-hide_banner", "-i", filepath.Join(dir, "source", "index.m3u8"),
		"-ss", fmt.Sprintf("%.3f", (from+6*time.Second).Seconds()), "-i", movie,
		"-filter_complex", "[0:v]select='eq(n,150)',setpts=PTS-STARTPTS,crop=1000:560:100:240[a];[1:v]setpts=PTS-STARTPTS,scale=1008:567,crop=1000:560:4:4[b];[a][b]psnr",
		"-frames:v", "1", "-f", "null", "-").CombinedOutput()
	if err != nil {
		t.Fatalf("compare: %v %s", err, out)
	}
	m := regexp.MustCompile(`average:([\d.]+)`).FindSubmatch(out)
	if m == nil {
		t.Fatalf("no PSNR in %s", out)
	}
	psnr, _ := strconv.ParseFloat(string(m[1]), 64)
	t.Logf("the panel is %.1f dB from the program", psnr)
	if psnr < 25 {
		t.Errorf("the panel is %.1f dB from the program", psnr)
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

// validatePackage checks a rendered package: it holds its renditions and its master playlist
// and nothing else, the validator finds no error in the playlist, playback reads it like a
// media file's package, and the video is frames long.
func validatePackage(t *testing.T, dir string, frames int) {
	t.Helper()
	validateRenditions(t, dir, frames, 1)
}

// validateRenditions is validatePackage for a package of audio renditions: a bulletin's
// carries one at each slot loudness besides its own sound.
func validateRenditions(t *testing.T, dir string, frames, audio int) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !e.IsDir() && e.Name() != "master.m3u8" {
			t.Errorf("%s holds %s, which is not a rendition", filepath.Base(dir), e.Name())
		}
	}
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
	if p.Source == nil || len(p.Stereo) != audio {
		t.Fatalf("%s: not read as a package: %+v", dir, p)
	}
	if got := p.Source.Info.Duration * couchtv.FPS; got < float64(frames)-0.01 || got > float64(frames)+0.01 {
		t.Errorf("%s: %.3f frames of video, want %d", dir, got, frames)
	}
	if p.Source.Info.Codec != "avc1.640028" || p.Source.Info.Width != 1920 || p.Source.Info.Height != 1080 {
		t.Errorf("%s: %+v", dir, p.Source.Info)
	}
}
