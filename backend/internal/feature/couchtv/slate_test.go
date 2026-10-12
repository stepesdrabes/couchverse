package couchtv

import (
	"context"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"couchverse/internal/feature/playback"
	"couchverse/internal/media/mp4"
)

// checkLoop plays copies 0 to n-1 of a loop: each starts where the one before
// ended, as long as the loop's segment lengths say, and can be cut anywhere.
func checkLoop(t *testing.T, name string, l *loop, n int) {
	t.Helper()
	scale := float64(l.in.Tracks[0].Timescale)
	var end uint64
	for k := range n {
		data, err := l.copy(k, 0)
		if err != nil {
			t.Fatal(err)
		}
		frags, err := mp4.ParseSegment(data, l.in)
		if err != nil {
			t.Fatal(err)
		}
		if !frags[0].FirstSync {
			t.Errorf("%s copy %d does not start on a sync sample", name, k)
		}
		// every fragment (ffmpeg writes one per GOP) starts where the one before ended
		var length uint64
		for i, f := range frags {
			if (k > 0 || i > 0) && f.BaseDecodeTime != end {
				t.Errorf("%s copy %d fragment %d starts at %d, the one before ends at %d", name, k, i, f.BaseDecodeTime, end)
			}
			end = f.BaseDecodeTime + f.Duration
			length += f.Duration
		}
		if got := float64(length) / scale; math.Abs(got-l.durations[k%len(l.durations)]) > 1e-6 {
			t.Errorf("%s copy %d lasts %.6fs, listed %.6fs", name, k, got, l.durations[k%len(l.durations)])
		}
		for _, keep := range []float64{1, 0.3} {
			cut, err := l.copy(k, keep)
			if err != nil {
				t.Fatal(err)
			}
			frags, err := mp4.ParseSegment(cut, l.in)
			if err != nil {
				t.Fatal(err)
			}
			if got := float64(frags[0].Duration) / scale; math.Abs(got-keep) > 0.03 {
				t.Errorf("%s copy %d cut to %g lasts %.3fs", name, k, keep, got)
			}
		}
	}
}

// TestSlate makes the slate and checks that its copies form one continuous
// stream per track and can be cut anywhere.
func TestSlate(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not installed")
	}
	data := t.TempDir()
	s := NewSlate(data, "ffmpeg")
	if err := s.Ensure(context.Background()); err != nil {
		t.Fatal(err)
	}
	f := s.Fill()
	if f.Video.durations[0] != 2 || f.Video.info.Width != 1920 || f.Video.info.Height != 1080 || f.Audio.info.Channels != "2" {
		t.Errorf("slate %+v %+v", f.Video.info, f.Audio.info)
	}
	checkLoop(t, "slate video", f.Video, 3)
	checkLoop(t, "slate audio", f.Audio, 3)
	// a second process reads the slate made before
	again := NewSlate(data, "false")
	if err := again.Ensure(context.Background()); err != nil || again.Fill().Audio.durations[0] != f.Audio.durations[0] {
		t.Errorf("reading the slate back: %v", err)
	}
}

// TestLoopPasses loops a rendition of two segments, as the standby card is: the
// copies run on across passes.
func TestLoopPasses(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not installed")
	}
	dir := filepath.Join(t.TempDir(), "source")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	args := append(playback.InputArgs("color=c=0x202020:s=64x36:r=25:d=8", "-f", "lavfi"),
		playback.RenditionArgs(dir, []string{"-map", "0:v", "-c:v", "libx264", "-g", "50", "-bf", "0", "-pix_fmt", "yuv420p"}, true)...)
	if out, err := exec.Command("ffmpeg", args...).CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	if _, err := playback.DescribeRendition(dir, playback.RenditionInfo{}); err != nil {
		t.Fatal(err)
	}
	l, err := openLoop(dir, playback.RenditionInfo{})
	if err != nil {
		t.Fatal(err)
	}
	if len(l.names) != 2 || math.Abs(l.seconds()-8) > 0.001 {
		t.Fatalf("loop of %d segments, %.3fs", len(l.names), l.seconds())
	}
	checkLoop(t, "card", l, 3*len(l.names))
}
