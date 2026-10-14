package media

import (
	"context"
	"math"
	"os/exec"
	"path/filepath"
	"testing"
)

// profile is a loudness track of per-second levels (each held for a second).
func profile(levels ...float64) *Loudness {
	l := &Loudness{}
	for _, v := range levels {
		for range 10 {
			l.Momentary = append(l.Momentary, v)
		}
	}
	return l
}

func repeat(v float64, n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = v
	}
	return out
}

func TestSteady(t *testing.T) {
	var levels []float64
	// dialogue: swinging between speech and pauses
	for i := range 40 {
		levels = append(levels, []float64{-25, -45}[i%2])
	}
	// music: loud and level for 60 s, then quiet music, then silence
	levels = append(levels, repeat(-18, 60)...)
	levels = append(levels, repeat(-40, 30)...)
	levels = append(levels, repeat(-120.7, 20)...)
	got := profile(levels...).Steady()
	if len(got) != 1 || got[0].Start != 40 || got[0].End != 100 || got[0].Loudness != -18 {
		t.Errorf("stretches %+v, want the music at 40-100 s", got)
	}
}

func TestLoudest(t *testing.T) {
	levels := append(append(repeat(-30, 20), repeat(-12, 10)...), repeat(-30, 30)...)
	l := profile(levels...)
	if got := l.Loudest(0, 60, 10); got != 20 {
		t.Errorf("loudest 10 s at %v, want 20", got)
	}
	if got := l.Loudest(25, 60, 10); got != 25 {
		t.Errorf("loudest 10 s from 25 at %v, want 25", got)
	}
	if got := l.Loudest(5, 12, 10); got != 5 {
		t.Errorf("a span shorter than the window starts at %v", got)
	}
}

func TestCombineLoudness(t *testing.T) {
	if got := CombineLoudness([]float64{-20, -20}, []float64{1, 3}); math.Abs(got+20) > 1e-9 {
		t.Errorf("equal programs combine to %v", got)
	}
	// energies average: a program 10 LU louder dominates
	if got := CombineLoudness([]float64{-30, -20}, []float64{1, 1}); math.Abs(got+22.596) > 0.01 {
		t.Errorf("combined %v, want about -22.6", got)
	}
	if got := CombineLoudness(nil, nil); got != -70 {
		t.Errorf("nothing combines to %v", got)
	}
}

func ffmpeg(t *testing.T, args ...string) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not installed")
	}
	if out, err := exec.Command("ffmpeg", append([]string{"-y", "-hide_banner", "-loglevel", "error"}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg: %v %s", err, out)
	}
}

// FullRange makes luma what a gray decode gives, 255 for a limited-range white, and
// stretches the chroma with it; without it the planes are the file's.
func TestFramesFullRange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "white.mp4")
	ffmpeg(t, "-f", "lavfi", "-i", "color=c=white:size=64x36:rate=5:duration=1", "-c:v", "libx264", "-pix_fmt", "yuv420p", path)
	luma := func(o FrameOptions) (y, u int) {
		o.Width, o.Height = 32, 18
		err := Frames(context.Background(), "ffmpeg", path, o, func(f Frame) error {
			y = int(f.Y[0])
			if len(f.U) > 0 {
				u = int(f.U[0])
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return y, u
	}
	if y, u := luma(FrameOptions{Chroma: true}); y < 233 || y > 237 || u < 126 || u > 130 {
		t.Errorf("limited range: luma %d, chroma %d, want about 235 and 128", y, u)
	}
	if y, u := luma(FrameOptions{Chroma: true, FullRange: true}); y < 253 || u < 126 || u > 130 {
		t.Errorf("full range: luma %d, chroma %d, want about 255 and 128", y, u)
	}
	if y, _ := luma(FrameOptions{}); y < 253 {
		t.Errorf("gray: luma %d, want about 255", y)
	}
}

func TestMeasurements(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clip.mp4")
	// 6 s: a test pattern with a keyframe every second, a tone that is silent for the
	// first two seconds
	ffmpeg(t, "-f", "lavfi", "-i", "testsrc2=size=320x180:rate=10:duration=6",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=6:sample_rate=48000,volume='if(lt(t,2),0,1)':eval=frame",
		"-c:v", "libx264", "-g", "10", "-pix_fmt", "yuv420p", "-c:a", "aac", path)
	ctx := context.Background()

	l, err := MeasureLoudness(ctx, "ffmpeg", path, 1, 4)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(l.Momentary); n < 38 || n > 41 {
		t.Errorf("%d momentary values for 4 s", n)
	}
	if l.Momentary[2] > -70 || l.Momentary[len(l.Momentary)-1] < -30 {
		t.Errorf("silence then the tone: %v ... %v", l.Momentary[2], l.Momentary[len(l.Momentary)-1])
	}
	if l.Integrated < -30 || l.Integrated > -10 {
		t.Errorf("integrated %v", l.Integrated)
	}

	var times []float64
	err = Frames(ctx, "ffmpeg", path, FrameOptions{From: 1, Length: 3, FPS: 2, Width: 64, Height: 36, Chroma: true},
		func(f Frame) error {
			if len(f.Y) != 64*36 || len(f.U) != 32*18 || len(f.V) != 32*18 {
				t.Errorf("planes %d %d %d", len(f.Y), len(f.U), len(f.V))
			}
			times = append(times, f.Time)
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if len(times) != 6 || math.Abs(times[0]-1) > 0.01 || math.Abs(times[5]-3.5) > 0.01 {
		t.Errorf("2 fps from 1 s: %v", times)
	}

	times = nil
	err = Frames(ctx, "ffmpeg", path, FrameOptions{From: 0.5, Keyframes: true, Width: 64, Height: 36},
		func(f Frame) error {
			times = append(times, f.Time)
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if len(times) != 5 || math.Abs(times[0]-1) > 0.01 || math.Abs(times[4]-5) > 0.01 {
		t.Errorf("keyframes from 0.5 s: %v", times)
	}

	if !HasChromaprint("ffmpeg") {
		t.Log("ffmpeg has no chromaprint muxer; fingerprints not tested")
		return
	}
	fp, err := Fingerprint(ctx, "ffmpeg", path, 0, 6)
	if err != nil {
		t.Fatal(err)
	}
	// 6 s is about 48 items of 0.1238 s, less the frames chromaprint spends warming up
	// (27 with Ubuntu's ffmpeg)
	if n := len(fp); n < 20 || n > 50 {
		t.Errorf("%d fingerprint items for 6 s", n)
	}
}
