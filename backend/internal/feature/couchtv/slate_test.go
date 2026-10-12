package couchtv

import (
	"context"
	"math"
	"os/exec"
	"testing"

	"couchverse/internal/media/mp4"
)

// TestSlate makes the slate and checks that its copies form one continuous
// stream per track and can be cut anywhere.
func TestSlate(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not installed")
	}
	s := NewSlate(t.TempDir(), "ffmpeg")
	if err := s.Ensure(context.Background()); err != nil {
		t.Fatal(err)
	}
	info := s.info()
	if info.VideoSeconds != 2 || info.Video.Width != 1920 || info.Video.Height != 1080 || info.Audio.Channels != "2" {
		t.Errorf("slate %+v", info)
	}
	for _, track := range []string{"video", "audio"} {
		init, err := s.Init(track)
		if err != nil {
			t.Fatal(err)
		}
		in, err := mp4.ParseInit(init)
		if err != nil {
			t.Fatal(err)
		}
		scale := float64(in.Tracks[0].Timescale)
		var end uint64
		for n := range 3 {
			data, err := s.Segment(track, n, 0)
			if err != nil {
				t.Fatal(err)
			}
			frags, err := mp4.ParseSegment(data, in)
			if err != nil {
				t.Fatal(err)
			}
			if n > 0 && frags[0].BaseDecodeTime != end || !frags[0].FirstSync {
				t.Errorf("%s copy %d starts at %d, the one before ends at %d", track, n, frags[0].BaseDecodeTime, end)
			}
			end = frags[0].BaseDecodeTime + frags[0].Duration
			for _, keep := range []float64{1, 0.3} {
				cut, err := s.Segment(track, n, keep)
				if err != nil {
					t.Fatal(err)
				}
				frags, err := mp4.ParseSegment(cut, in)
				if err != nil {
					t.Fatal(err)
				}
				if got := float64(frags[0].Duration) / scale; math.Abs(got-keep) > 0.03 {
					t.Errorf("%s copy %d cut to %g lasts %.3fs", track, n, keep, got)
				}
			}
		}
	}
	// a second process reads the slate made before
	again := NewSlate(s.dir[:len(s.dir)-len("/cache/couchtv/slate")], "false")
	if err := again.Ensure(context.Background()); err != nil || again.info() != info {
		t.Errorf("reading the slate back: %v, %+v", err, again.info())
	}
}
