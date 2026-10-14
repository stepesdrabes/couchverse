package server_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"couchverse/internal/config"
	"couchverse/internal/feature/couchtv"
	"couchverse/internal/feature/jobs"
)

// syntheticSpeech writes a mono 16 kHz WAV of the given seconds with speech-like stretches
// (band-limited noise and a voice of harmonics swelling at a syllable rate) over a quiet room:
// what the scene picker's voice-activity detector takes for speech.
func syntheticSpeech(t *testing.T, path string, seconds float64, speech [][2]float64) {
	t.Helper()
	const rate = 16000
	rng := rand.New(rand.NewSource(11))
	n := int(seconds * rate)
	samples := make([]int16, n)
	var low float64
	for i := range samples {
		tm := float64(i) / rate
		white := rng.Float64()*2 - 1
		low += 0.3 * (white - low)
		v := 0.002 * white
		for _, s := range speech {
			if tm >= s[0] && tm < s[1] {
				env := math.Pow(0.5+0.5*math.Sin(2*math.Pi*4*(tm-s[0])-math.Pi/2), 1.5)
				pitch := 120 + 25*math.Sin(2*math.Pi*0.7*tm)
				voice := math.Sin(2*math.Pi*pitch*tm) + 0.6*math.Sin(2*math.Pi*2*pitch*tm) + 0.5*math.Sin(2*math.Pi*3*pitch*tm) +
					0.4*math.Sin(2*math.Pi*5*pitch*tm)
				v += 0.12 * env * (0.6*voice + 1.5*low)
			}
		}
		samples[i] = int16(math.Max(-1, math.Min(1, v)) * 32767)
	}
	var buf bytes.Buffer
	head := []any{[]byte("RIFF"), uint32(36 + 2*n), []byte("WAVE"), []byte("fmt "), uint32(16), uint16(1), uint16(1), uint32(rate),
		uint32(rate * 2), uint16(2), uint16(16), []byte("data"), uint32(2 * n)}
	for _, h := range head {
		if err := binary.Write(&buf, binary.LittleEndian, h); err != nil {
			t.Fatal(err)
		}
	}
	if err := binary.Write(&buf, binary.LittleEndian, samples); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestCouchTvTrailerAnalysis analyzes a movie that has two audio tracks, Czech as the default and
// English, and subtitles in both languages (the English cues over the English track's speech,
// the Czech ones saying something else): the scenes of its trailer are picked from the English
// track and its subtitles, whole lines cut in the pauses around them, and stored.
func TestCouchTvTrailerAnalysis(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not installed")
	}
	env := newTestEnv(t, func(cfg *config.Config) { cfg.FFmpegPath, cfg.FFprobePath = "ffmpeg", "ffprobe" })
	ctx := context.Background()
	w := newAnalysisWorker(t, env)

	// 20 minutes: a picture that does not move, a Czech track of a quiet tone (the default) and
	// an English one with eight exchanges of two lines between 80 s and 600 s, 70 s apart
	english := []string{"Who are you?", "Nobody leaves this world alive.", "What do you want?", "I want the truth about my family.",
		"Where is the key?", "Never trust a man who runs.", "Can you save him?", "There is a price for everything."}
	var speech [][2]float64
	var vtt, czech strings.Builder
	vtt.WriteString("WEBVTT\n\n")
	czech.WriteString("WEBVTT\n\n")
	for k := range 4 {
		at := 80 + 140*float64(k)
		// two lines of an exchange, each spoken a little inside its cue
		for j, start := range []float64{at, at + 1.9} {
			end := start + 1.5
			speech = append(speech, [2]float64{start + 0.1, end - 0.1})
			fmt.Fprintf(&vtt, "%s --> %s\n%s\n\n", ts(start), ts(end), english[2*k+j])
			fmt.Fprintf(&czech, "%s --> %s\nCZECH %d\n\n", ts(start), ts(end), 2*k+j)
		}
	}
	dir := t.TempDir()
	wav := filepath.Join(dir, "english.wav")
	syntheticSpeech(t, wav, 1200, speech)
	img := image.NewGray(image.Rect(0, 0, 320, 180))
	for y := 0; y < 180; y++ {
		for x := 0; x < 320; x++ {
			img.SetGray(x, y, color.Gray{Y: uint8(60 + x/2 + 20*((x/40+y/30)%2))})
		}
	}
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		t.Fatal(err)
	}
	pic := filepath.Join(dir, "picture.png")
	if err := os.WriteFile(pic, pngBuf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	lib, err := w.files.ManagedLibraryByKind(ctx, "movies")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(lib.Path, "Dialogue Test (2021)", "Dialogue Test (2021).mkv")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	if out, err := exec.Command("ffmpeg", "-y", "-hide_banner", "-loglevel", "error",
		"-loop", "1", "-framerate", "5", "-t", "1200", "-i", pic,
		"-f", "lavfi", "-i", "sine=frequency=220:sample_rate=48000:duration=1200", "-i", wav,
		"-map", "0:v", "-map", "1:a", "-map", "2:a", "-c:v", "libx264", "-preset", "ultrafast", "-tune", "stillimage", "-pix_fmt", "yuv420p", "-g", "50",
		"-c:a", "aac", "-ar", "16000", "-b:a", "32k", "-metadata:s:a:0", "language=ces", "-metadata:s:a:1", "language=eng",
		"-disposition:a:0", "default", "-disposition:a:1", "0", path).CombinedOutput(); err != nil {
		t.Fatalf("generate: %v %s", err, out)
	}
	t.Logf("generated in %s", time.Since(started).Round(time.Millisecond))
	fileID := w.ingest(ctx, path)
	w.run(ctx, "probe")
	movieID := w.titleID(ctx, "Dialogue Test (2021)")

	var englishStream, czechStream int
	if err := env.pool.QueryRow(ctx, `SELECT stream_index FROM audio_streams WHERE media_file_id = $1 AND lang IN ('en', 'eng')`, fileID).Scan(&englishStream); err != nil {
		t.Fatalf("the English stream: %v", err)
	}
	if err := env.pool.QueryRow(ctx, `SELECT stream_index FROM audio_streams WHERE media_file_id = $1 AND is_default`, fileID).Scan(&czechStream); err != nil || czechStream == englishStream {
		t.Fatalf("the default stream %d, the English %d (%v)", czechStream, englishStream, err)
	}
	for lang, text := range map[string]string{"en": vtt.String(), "cs": czech.String()} {
		rel := filepath.Join("subtitles", fileID, lang+".vtt")
		if err := os.MkdirAll(filepath.Dir(filepath.Join(env.cfg.DataDir, rel)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(env.cfg.DataDir, rel), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := env.pool.Exec(ctx, `INSERT INTO subtitles (media_file_id, lang, label, source, forced, path) VALUES ($1, $2, $2, 'uploaded', false, $3)`,
			fileID, lang, rel); err != nil {
			t.Fatal(err)
		}
	}

	started = time.Now()
	if err := w.couchtv.Handle(ctx, &jobs.Job{Payload: json.RawMessage(`{"titleId":"` + movieID + `","force":true}`)}, func(int) {}); err != nil {
		t.Fatal(err)
	}
	t.Logf("analyzed in %s", time.Since(started).Round(time.Millisecond))
	var raw []byte
	var source *string
	var version int
	if err := env.pool.QueryRow(ctx, `SELECT trailer, trailer_source, trailer_version FROM couchtv_titles WHERE title_id = $1`, movieID).Scan(&raw, &source, &version); err != nil {
		t.Fatal(err)
	}
	var scenes []couchtv.CouchTvTrailerScene
	if err := json.Unmarshal(raw, &scenes); err != nil {
		t.Fatal(err)
	}
	if source == nil || *source != "auto" || version != couchtv.TrailerVersion || len(scenes) < 3 || len(scenes) > 4 {
		t.Fatalf("%d scenes from %v at version %d: %s", len(scenes), source, version, raw)
	}
	for i, s := range scenes {
		// the picture and the original-language track, whatever the file's default is
		if s.MediaFileID != fileID || s.AudioFileID != fileID || s.AudioStream == nil || *s.AudioStream != englishStream || s.Source != "subtitles" {
			t.Errorf("scene %d: %+v", i, s)
		}
		if s.DurationMs < 3000 || s.DurationMs > 8000 || s.DurationMs%40 != 0 || s.StartMs < 60_000 || s.StartMs > 600_000 {
			t.Errorf("scene %d: %d ms from %d", i, s.DurationMs, s.StartMs)
		}
		if len(s.Lines) == 0 {
			t.Fatalf("scene %d has no lines", i)
		}
		// every line plays whole inside the scene, with a pause before the first and after the last
		if first, last := s.Lines[0], s.Lines[len(s.Lines)-1]; first.FromMs < 150 || last.ToMs > s.DurationMs-250 {
			t.Errorf("scene %d: lines from %d to %d ms of %d", i, first.FromMs, last.ToMs, s.DurationMs)
		}
		for _, l := range s.Lines {
			if l.Text == "" || strings.Contains(l.Text, "CZECH") || !slicesContains(english, l.Text) {
				t.Errorf("scene %d: the line %q is not from the English subtitles", i, l.Text)
			}
			if l.FromMs < 0 || l.ToMs > s.DurationMs || l.ToMs-l.FromMs < 250 {
				t.Errorf("scene %d: line %+v of %d ms", i, l, s.DurationMs)
			}
		}
		if i > 0 && s.StartMs-scenes[i-1].StartMs < 120_000 {
			t.Errorf("scenes %d and %d are %d ms apart", i-1, i, s.StartMs-scenes[i-1].StartMs)
		}
	}
	// a title with an admin's trailer keeps it
	if _, err := env.pool.Exec(ctx, `UPDATE couchtv_titles SET trailer_source = 'admin', trailer = '[]' WHERE title_id = $1`, movieID); err != nil {
		t.Fatal(err)
	}
	if err := w.couchtv.Handle(ctx, &jobs.Job{Payload: json.RawMessage(`{"titleId":"` + movieID + `","force":true}`)}, func(int) {}); err != nil {
		t.Fatal(err)
	}
	if err := env.pool.QueryRow(ctx, `SELECT trailer, trailer_source FROM couchtv_titles WHERE title_id = $1`, movieID).Scan(&raw, &source); err != nil || string(raw) != "[]" || *source != "admin" {
		t.Errorf("the admin's trailer after a new analysis: %s from %v (%v)", raw, source, err)
	}
}

func ts(seconds float64) string {
	return fmt.Sprintf("%02d:%02d:%06.3f", int(seconds)/3600, int(seconds)/60%60, math.Mod(seconds, 60))
}

func slicesContains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
