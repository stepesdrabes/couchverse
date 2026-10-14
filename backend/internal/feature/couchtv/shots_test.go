package couchtv

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestShotsLocal runs the shots analysis on local files and prints its picks: no
// fixtures, so it only runs when pointed at some.
//
//	COUCHVERSE_SHOTS_SOURCE=a.mp4[,b.mp4...] COUCHVERSE_SHOTS_KIND=movie|series \
//	  COUCHVERSE_SHOTS_OUT=picks.json go test ./internal/feature/couchtv -run TestShotsLocal -v
func TestShotsLocal(t *testing.T) {
	src := os.Getenv("COUCHVERSE_SHOTS_SOURCE")
	if src == "" {
		t.Skip("COUCHVERSE_SHOTS_SOURCE names no file")
	}
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is not installed")
	}
	var files []titleFile
	for i, p := range strings.Split(src, ",") {
		out, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of",
			"default=nw=1:nk=1", p).Output()
		if err != nil {
			t.Fatal(err)
		}
		d, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, titleFile{ID: fmt.Sprintf("file%d", i), Path: p, Duration: d, Readable: true})
	}
	kind := os.Getenv("COUCHVERSE_SHOTS_KIND")
	if kind == "" {
		kind = "movie"
	}
	a := &Analyzer{FFmpegPath: ffmpeg}
	if os.Getenv("COUCHVERSE_SHOTS_ALL") != "" {
		stretches := 12
		if kind == "series" {
			stretches = 4
		}
		for i, f := range files {
			cands, err := a.fileShots(context.Background(), f, i, stretches)
			if err != nil {
				t.Fatal(err)
			}
			for _, c := range cands {
				t.Logf("candidate %s @%.2f s, %d ms, score %.3f, risk %.2f, avoid %v", c.shot.MediaFileID, c.t, c.shot.DurationMs, c.shot.Score, c.risk, c.avoid)
			}
		}
	}
	start := time.Now()
	shots := a.shots(context.Background(), kind, files)
	t.Logf("%d shots in %v", len(shots), time.Since(start).Round(time.Millisecond))
	for _, s := range shots {
		t.Logf("%s @%.2f s, %d ms, score %.3f", s.MediaFileID, float64(s.StartMs)/1000, s.DurationMs, s.Score)
	}
	if out := os.Getenv("COUCHVERSE_SHOTS_OUT"); out != "" {
		b, err := json.MarshalIndent(shots, "", " ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(out, b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// A stretch's pick weighs a still's picture: a contrasty close-up loses to a plain wide.
func TestPickStillsWeighsRisk(t *testing.T) {
	stills := []still{
		{t: 1, mean: 100, p95: 200, contrast: 50, risk: closeUpRisk},
		{t: 2, mean: 100, p95: 200, contrast: 30},
		{t: 12, mean: 100, p95: 200, contrast: 40, risk: 0.2},
		{t: 14, mean: 100, p95: 200, contrast: 36},
		{t: 25, mean: 100, p95: 200, contrast: 50, risk: 1},
	}
	got := pickStills(stills, 0, 30, 3)
	if len(got) != 2 || got[0].t != 2 || got[1].t != 14 {
		t.Errorf("picked %+v, want the best ranked of the first two stretches and none of the last", got)
	}
}

func TestShotRating(t *testing.T) {
	for _, tt := range []struct{ score, risk, want float64 }{
		{0.8, 0, 0.8},
		{0.8, closeUpRisk, 0.24},
		{1, 0.2, 0.8},
		{0.5, 1, 0},
	} {
		if got := shotRating(tt.score, tt.risk); math.Abs(got-tt.want) > 1e-9 {
			t.Errorf("rating of %v at risk %v: %v, want %v", tt.score, tt.risk, got, tt.want)
		}
	}
}

// A shot to avoid never airs, even when it is the best of the title; a close-up may, and so may
// a shot with one gory frame, which only rates it low.
func TestChooseShotsAvoidsRisk(t *testing.T) {
	cand := func(start int, score, risk float64, avoid bool) candidate {
		return candidate{shot: CouchTvShot{MediaFileID: "f", StartMs: start, Score: score}, t: float64(start), risk: risk, avoid: avoid}
	}
	got := chooseShots([]candidate{
		cand(1000, shotRating(1, 1), 1, true),
		cand(2000, shotRating(0.9, closeUpRisk), closeUpRisk, false),
		cand(3000, shotRating(0.4, 0), 0, false),
		cand(4000, shotRating(0.8, avoidRisk), avoidRisk, true),
		cand(5000, shotRating(0.8, 0), 0, true), // gory between the checks: nothing rates it
		cand(6000, shotRating(0.9, 1), 1, false),
	}, 6)
	var order []int
	for _, s := range got {
		order = append(order, s.StartMs)
	}
	if fmt.Sprint(order) != "[2000 3000 6000]" {
		t.Errorf("chose %v, want the close-up, the plain shot and the one with a stray frame, in airing order", order)
	}
	if len(got) == 3 && (got[0].Score <= got[2].Score || got[1].Score <= got[0].Score) {
		t.Errorf("scores %v: the plain shot should rate above the close-up, and the close-up above the stray frame", got)
	}
	if got := chooseShots([]candidate{cand(1000, 0, 1, true)}, 6); got == nil || len(got) != 0 {
		t.Errorf("only a gory shot chose %v, want none (the promo falls back to artwork)", got)
	}
}
