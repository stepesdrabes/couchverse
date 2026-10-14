package couchtv

import (
	"context"
	"fmt"
	"math"
	"slices"

	"couchverse/internal/media"
)

// The picture checks of a trailer scene (D43), on cheap decodes like the footage shots': 160 x 90
// frames at 25 fps around the candidate, so a cut lands on its frame. They find where the shots
// change (a scene opens on a fresh shot and does not end in a flash of the next), and refuse
// scenes that are dark, washed out, flat, a montage or too violent to air.

// cutJump is how far a frame's difference from the one before must jump over the one before
// that (percent of full scale, as the shots' scene detection scores it) to be a cut.
const cutJump = 10.0

// What a scene's picture must be. It needs less than a promo shot: dialogue is often shot dim
// and flat, and a scene is judged as a whole, not picked for its looks among thousands.
const (
	// a frame is usable when its brightest 5 % reach sceneP95, its mean is within the two bounds
	// and its contrast (luma 16 to 235) reaches sceneContrast; the first and last frame of a
	// scene must not be black: their brightest 5 % reach sceneBlack
	sceneP95, sceneMeanLow, sceneMeanHigh, sceneContrast = 44, 22, 215, 9
	sceneBlack                                           = 34
	// a scene's frames are usable (not black, washed out or flat) at least this often
	usableShare = 0.8
	// a shot shorter than this on average is a montage
	montageShot = 1.0
	// a scene whose frames average this luma is lit well enough
	lightFull = 70.0
	// risk is a scene's violence score (see Analyzer.Risk) from which it is refused
	riskLimit = 0.5
)

// SceneRisk rates how unfit for the channel a scene's picture is, 0 (fine) to 1 (violent), from
// its frames at 160 x 90 with chroma. The analysis takes none by default.
type SceneRisk func(frames []media.Frame) float64

// pictureScan is what the checks read of a stretch of frames.
type pictureScan struct {
	frames []media.Frame
	stills []still
	// diff is each frame's mean absolute difference from the one before (percent of full scale)
	diff []float64
	// Cuts are the times where a new shot starts.
	Cuts []float64
}

// scanPicture decodes [from, from+length) of a file and finds its cuts.
func scanPicture(ctx context.Context, ffmpeg, path string, from, length float64) (*pictureScan, error) {
	s := &pictureScan{}
	err := media.Frames(ctx, ffmpeg, path, media.FrameOptions{From: from, Length: length, FPS: FPS,
		Width: shotW, Height: shotH, Chroma: true}, func(f media.Frame) error {
		f.Y, f.U, f.V = slices.Clone(f.Y), slices.Clone(f.U), slices.Clone(f.V)
		d := 0.0
		if n := len(s.frames); n > 0 {
			d = frameDiff(s.frames[n-1], f)
		}
		s.frames = append(s.frames, f)
		s.stills = append(s.stills, readStill(f))
		s.diff = append(s.diff, d)
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.findCuts()
	return s, nil
}

func (s *pictureScan) findCuts() {
	for i := 2; i < len(s.diff); i++ {
		if min(s.diff[i], math.Abs(s.diff[i]-s.diff[i-1])) >= cutJump {
			s.Cuts = append(s.Cuts, s.frames[i].Time)
		}
	}
}

// pictureCheck is how a placed scene's frames fare.
type pictureCheck struct {
	// Score is 0 to 1: contrast and motion, as the footage shots are rated.
	Score float64
	// Why says what refused the scene; empty when it passed.
	Why string
}

// check rates the frames of [start, end).
func (s *pictureScan) check(start, end float64, risk SceneRisk) pictureCheck {
	lo, hi := -1, -1
	for i, f := range s.frames {
		if f.Time >= start-1e-6 && lo < 0 {
			lo = i
		}
		if f.Time < end-1e-6 {
			hi = i
		}
	}
	if lo < 0 || hi < lo+int(sceneMin*FPS)-5 {
		return pictureCheck{Why: "no frames"}
	}
	frames := s.frames[lo : hi+1]
	usable, contrast, motion, light := 0, 0.0, 0.0, 0.0
	for i, st := range s.stills[lo : hi+1] {
		if st.p95 >= sceneP95 && st.mean >= sceneMeanLow && st.mean <= sceneMeanHigh && st.contrast >= sceneContrast {
			usable++
		}
		contrast += st.contrast
		light += st.mean
		if i > 0 {
			motion += s.diff[lo+i]
		}
	}
	n := float64(len(frames))
	contrast, motion, light = contrast/n, motion/n, light/n
	switch {
	case s.stills[lo].p95 < sceneBlack || s.stills[hi].p95 < sceneBlack:
		return pictureCheck{Why: "it opens or ends on black"}
	case float64(usable)/n < usableShare:
		return pictureCheck{Why: fmt.Sprintf("dark, flat or washed out (%.0f %% of its frames are fine)", 100*float64(usable)/n)}
	}
	shots := 1
	for _, c := range s.Cuts {
		if c > start+frameSec && c < end-frameSec {
			shots++
		}
	}
	if (end-start)/float64(shots) < montageShot {
		return pictureCheck{Why: "a montage"}
	}
	// contrast and motion as the footage shots are rated, and light: a dim scene is a poor picture
	score := 0.4*min(contrast/contFull, 1) + 0.3*min(motion/motionFull, 1) + 0.3*min(light/lightFull, 1)
	if risk != nil {
		if r := risk(frames); r >= riskLimit {
			return pictureCheck{Why: "too violent"}
		} else {
			score -= 0.5 * r
		}
	}
	return pictureCheck{Score: math.Max(0, score)}
}
