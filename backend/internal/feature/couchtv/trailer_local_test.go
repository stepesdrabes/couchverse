package couchtv

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"couchverse/internal/media"
)

// TestTrailerPickLocal runs the scene picker on local files, for judging its picks. It needs
// ffmpeg and COUCHVERSE_TRAILER_SOURCE, a media file (several, comma-separated, for a series'
// episodes). Optional:
//
//	COUCHVERSE_TRAILER_SUBS      WebVTT files, comma-separated per source, tracks of one source joined by +
//	COUCHVERSE_TRAILER_STREAM    the audio stream's index (ffprobe's), for all sources; default track otherwise
//	COUCHVERSE_TRAILER_KEYWORDS  the title's own words, comma-separated
//	COUCHVERSE_TRAILER_AVOID     stretches of the source, "from-to" seconds comma-separated, that no scene may touch:
//	                             an editor's hand until a violence check (SceneRisk) is wired in
//	COUCHVERSE_TRAILER_OUT       a file for the scenes as JSON
//
// It prints the picker's trace and the chosen scenes as a script, then as JSON.
func TestTrailerPickLocal(t *testing.T) {
	sources := os.Getenv("COUCHVERSE_TRAILER_SOURCE")
	if sources == "" {
		t.Skip("set COUCHVERSE_TRAILER_SOURCE to a media file to run the scene picker on it")
	}
	subs := strings.Split(os.Getenv("COUCHVERSE_TRAILER_SUBS"), ",")
	var stream *int
	if s := os.Getenv("COUCHVERSE_TRAILER_STREAM"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil {
			t.Fatal(err)
		}
		stream = &n
	}
	var files []pickFile
	for i, path := range strings.Split(sources, ",") {
		out, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", path).Output()
		if err != nil {
			t.Fatal(err)
		}
		duration, _ := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
		id := "source-" + strconv.Itoa(i)
		f := pickFile{ID: id, Path: path, Duration: duration, Audio: dialogueTrack{FileID: id, Path: path, Stream: stream, Lang: "en"}}
		if i < len(subs) && subs[i] != "" {
			f.Subs = strings.Split(subs[i], "+")
		}
		files = append(files, f)
	}
	var avoid []timeSpan
	for _, r := range strings.Split(os.Getenv("COUCHVERSE_TRAILER_AVOID"), ",") {
		from, to, ok := strings.Cut(r, "-")
		a, errA := strconv.ParseFloat(from, 64)
		b, errB := strconv.ParseFloat(to, 64)
		if ok && errA == nil && errB == nil {
			avoid = append(avoid, timeSpan{a, b})
		}
	}
	p := &scenePicker{FFmpeg: "ffmpeg", Keywords: strings.Split(os.Getenv("COUCHVERSE_TRAILER_KEYWORDS"), ","),
		Trace: func(format string, args ...any) { t.Logf(format, args...) }}
	if len(avoid) > 0 {
		p.Risk = func(frames []media.Frame) float64 {
			last := frames[len(frames)-1].Time
			for _, a := range avoid {
				if a.overlaps(frames[0].Time, last) {
					return 1
				}
			}
			return 0
		}
	}
	start := time.Now()
	picked, err := p.pickTitle(context.Background(), files)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("picked %d scenes in %s", len(picked), time.Since(start).Round(time.Millisecond))
	scenes := []CouchTvTrailerScene{}
	for _, c := range picked {
		scenes = append(scenes, storedScene(c, files))
	}
	t.Logf("the trailer:\n%s", describeScenes(scenes))
	body, err := json.MarshalIndent(map[string]any{"scenes": scenes, "sources": files}, "", "\t")
	if err != nil {
		t.Fatal(err)
	}
	if out := os.Getenv("COUCHVERSE_TRAILER_OUT"); out != "" {
		if err := os.WriteFile(out, body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
