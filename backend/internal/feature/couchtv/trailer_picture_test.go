package couchtv

import (
	"slices"
	"testing"

	"couchverse/internal/media"
)

// frame is a 160 x 90 frame of the given luma: flat, or a left half and a right half; Time is
// the second it is at.
func frame(t, left, right float64) media.Frame {
	y := make([]byte, shotW*shotH)
	for i := range y {
		if i%shotW < shotW/2 {
			y[i] = byte(left)
		} else {
			y[i] = byte(right)
		}
	}
	return media.Frame{Time: t, Width: shotW, Height: shotH, Y: y}
}

// scanOf builds a scan from frames at 25 fps: pictures[i] gives the luma halves of frame i.
func scanOf(from float64, seconds float64, picture func(t float64) (left, right float64)) *pictureScan {
	s := &pictureScan{}
	for i := range int(seconds * FPS) {
		t := from + float64(i)/FPS
		l, r := picture(t)
		f := frame(t, l, r)
		d := 0.0
		if n := len(s.frames); n > 0 {
			d = frameDiff(s.frames[n-1], f)
		}
		s.frames = append(s.frames, f)
		s.stills = append(s.stills, readStill(f))
		s.diff = append(s.diff, d)
	}
	s.findCuts()
	return s
}

func TestPictureCuts(t *testing.T) {
	// two shots joined at 4.0 s and 7.2 s, a flicker of brightness that is no cut at 2.0 s
	scan := scanOf(0, 10, func(t float64) (float64, float64) {
		switch {
		case t < 4:
			return 90, 160
		case t < 7.2:
			return 150, 70
		}
		return 60, 190
	})
	if len(scan.Cuts) != 2 || scan.Cuts[0] < 3.96 || scan.Cuts[0] > 4.04 || scan.Cuts[1] < 7.16 || scan.Cuts[1] > 7.24 {
		t.Errorf("cuts %v, want 4.0 and 7.2", scan.Cuts)
	}
	// a slow fade is no cut
	fade := scanOf(0, 6, func(t float64) (float64, float64) { return 60 + 15*t, 150 + 8*t })
	if len(fade.Cuts) != 0 {
		t.Errorf("a fade has cuts %v", fade.Cuts)
	}
}

func TestPictureCheck(t *testing.T) {
	good := func(float64) (float64, float64) { return 80, 170 }
	scan := scanOf(100, 12, good)
	if c := scan.check(102, 107, nil); c.Why != "" || c.Score <= 0 {
		t.Errorf("a good picture: %+v", c)
	}
	// black or flat at an edge
	dark := scanOf(100, 12, func(t float64) (float64, float64) {
		if t >= 106 {
			return 17, 19
		}
		return 80, 170
	})
	if c := dark.check(102, 107, nil); c.Why == "" {
		t.Error("a scene that ends on black passed")
	}
	if c := dark.check(100, 105, nil); c.Why != "" {
		t.Errorf("the same picture before the black: %s", c.Why)
	}
	// dim all through
	dim := scanOf(100, 12, func(float64) (float64, float64) { return 20, 28 })
	if c := dim.check(102, 107, nil); c.Why == "" {
		t.Error("a dim scene passed")
	}
	// a montage: a cut every half second
	montage := scanOf(100, 12, func(t float64) (float64, float64) {
		if int(t*2)%2 == 0 {
			return 80, 170
		}
		return 170, 80
	})
	if c := montage.check(102, 107, nil); c.Why != "a montage" {
		t.Errorf("a montage: %q", c.Why)
	}
	// too few frames for a scene
	if c := scan.check(110, 115, nil); c.Why == "" {
		t.Error("a scene past the frames passed")
	}
	// the hook: a risk from its limit refuses, below it counts against the picture
	var seen []media.Frame
	risky := func(frames []media.Frame) float64 { seen = frames; return 0.6 }
	if c := scan.check(102, 107, risky); c.Why != "too violent" || len(seen) < 120 {
		t.Errorf("risk 0.6: %q over %d frames", c.Why, len(seen))
	}
	base := scan.check(102, 107, nil).Score
	if c := scan.check(102, 107, func([]media.Frame) float64 { return 0.2 }); c.Why != "" || c.Score >= base {
		t.Errorf("risk 0.2: %+v against %.2f", c, base)
	}
	if !slices.IsSortedFunc(scan.frames, func(a, b media.Frame) int {
		if a.Time < b.Time {
			return -1
		}
		return 1
	}) {
		t.Error("frames out of order")
	}
}
