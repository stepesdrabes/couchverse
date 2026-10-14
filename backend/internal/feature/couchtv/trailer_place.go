package couchtv

import (
	"fmt"
	"math"
)

// Placing a scene (D43): given the lines it plays, find where it starts and ends. A trailer
// never cuts into a line, so a scene starts in the pause before its first line, a little ahead
// of it (and on a fresh shot when a cut is near), and ends in the pause after its last, a
// little behind it (on the end of a shot when one is near).

// What a scene may be, in seconds.
const (
	sceneMin = 3.0
	sceneMax = 8.0
	// the lead before the first line and the tail after the last: ideal and bounds
	leadIdeal, leadMin, leadMax = 0.45, 0.10, 1.6
	tailIdeal, tailMin, tailMax = 0.60, 0.25, 1.6
	// a cut within this of the ideal edge is where the edge goes
	cutSnapLead, cutSnapTail = 1.0, 0.9
	// no cut inside a scene's first second, nor in the last half second
	openingShot, closingShot = 1.0, 0.5
)

// sceneWindow is the sound and picture around a candidate on one clock: seconds from the
// file's start.
type sceneWindow struct {
	// Origin is the time of the audio's first frame.
	Origin float64
	Audio  vadAnalysis
	// Cuts are where a new shot starts.
	Cuts []float64
}

func (w sceneWindow) frame(t float64) int { return int(math.Floor((t - w.Origin) / frameSec)) }

func (w sceneWindow) at(mask []bool, t float64) bool {
	i := w.frame(t)
	return i >= 0 && i < len(mask) && mask[i]
}

// covers reports whether the audio reaches [from, to].
func (w sceneWindow) covers(from, to float64) bool {
	return from >= w.Origin && to <= w.Origin+float64(len(w.Audio.Active))*frameSec
}

// first is the start of the first frame of the mask in [from, to), false if none.
func (w sceneWindow) first(mask []bool, from, to float64) (float64, bool) {
	for t := from; t < to; t += frameSec {
		if w.at(mask, t) {
			return t, true
		}
	}
	return 0, false
}

// last is the end of the last frame of the mask in [from, to), false if none.
func (w sceneWindow) last(mask []bool, from, to float64) (float64, bool) {
	for t := to - frameSec; t >= from; t -= frameSec {
		if w.at(mask, t) {
			return t + frameSec, true
		}
	}
	return 0, false
}

// minPause is the shortest pause a scene is cut in: the lead and the tail need room.
const minPause = 0.22

// pause is a stretch where nothing stands out over the background. A pause that reaches the edge
// of the window it was found in may go on beyond it: openLeft and openRight say so.
type pause struct {
	timeSpan
	openLeft, openRight bool
}

// pauses lists the pauses of at least minPause seconds in [from, to).
func (w sceneWindow) pauses(from, to float64) []pause {
	var out []pause
	run := -1
	lo, hi := max(0, w.frame(from)), min(len(w.Audio.Quiet), w.frame(to))
	for i := lo; i <= hi; i++ {
		quiet := i < hi && w.Audio.Quiet[i]
		switch {
		case quiet && run < 0:
			run = i
		case !quiet && run >= 0:
			if float64(i-run)*frameSec >= minPause {
				out = append(out, pause{timeSpan{w.Origin + float64(run)*frameSec, w.Origin + float64(i)*frameSec}, run == lo, i == hi})
			}
			run = -1
		}
	}
	return out
}

// speechShare is the share of [from, to) the detector takes for speech.
func (w sceneWindow) speechShare(from, to float64) float64 {
	return w.Audio.speechIn(from-w.Origin, to-w.Origin)
}

// cutsIn lists the cuts in (from, to].
func (w sceneWindow) cutsIn(from, to float64) []float64 {
	var out []float64
	for _, c := range w.Cuts {
		if c > from && c <= to {
			out = append(out, c)
		}
	}
	return out
}

// scenePlace is where a scene starts and ends, and where its speech is heard to start and end.
type scenePlace struct {
	Start, End    float64
	Onset, Offset float64
}

// fit rates how comfortably the scene is cut around its lines, 0 to 1: a lead under a quarter of
// a second or a tail under a third of one makes the cuts feel abrupt.
func (p scenePlace) fit() float64 {
	f := 1.0
	if p.Onset-p.Start < 0.25 {
		f -= 0.08
	}
	if p.End-p.Offset < 0.35 {
		f -= 0.08
	}
	return f
}

// placeScene finds a scene for lines spoken in [from, to] (as the subtitles or the detector
// say). The sound decides where it can be cut: a scene starts in the pause that ends where the
// first line is heard to start and ends in the pause that starts where the last line is heard
// to stop. It fails, saying why, when there is no such pause (the line before or after runs on
// into it), when the sound does not bear the lines out, or when no scene of 3 to 8 s fits.
// blocked are stretches the scene must not touch (lyrics).
func placeScene(from, to float64, w sceneWindow, blocked []timeSpan) (p scenePlace, why string) {
	if !w.covers(from-3.5, to+3.5) {
		return p, "the sound does not reach it"
	}
	// the pause before ends where the first line is heard to start; one that goes on past the
	// window has not ended
	before := w.pauses(from-3.0, from+0.8)
	var pre *pause
	for i := range before {
		if !before[i].openRight && before[i].To >= from-0.6 && (pre == nil || math.Abs(before[i].To-(from+0.1)) < math.Abs(pre.To-(from+0.1))) {
			pre = &before[i]
		}
	}
	if pre == nil {
		return p, "no pause before the first line"
	}
	after := w.pauses(to-0.8, to+3.0)
	var post *pause
	for i := range after {
		if !after[i].openLeft && after[i].From <= to+0.6 && (post == nil || math.Abs(after[i].From-(to-0.25)) < math.Abs(post.From-(to-0.25))) {
			post = &after[i]
		}
	}
	if post == nil {
		return p, "no pause after the last line"
	}
	onset, offset := pre.To, post.From
	if offset-onset < 0.5 || w.speechShare(onset, offset) < 0.2 {
		return p, "nothing is heard where the lines are"
	}

	lo, hi := math.Max(pre.From+frameSec, onset-leadMax), onset-leadMin
	if lo > hi {
		return p, "the pause before the first line is too short"
	}
	start := math.Min(math.Max(onset-leadIdeal, lo), hi)
	if c, ok := nearest(w.cutsIn(lo-frameSec, hi), onset-leadIdeal, cutSnapLead); ok {
		start = c
	}
	// the scene opens on its shot: a cut in its first second moves the start up to it
	for range 4 {
		next := w.cutsIn(start+frameSec, start+openingShot)
		if len(next) == 0 {
			break
		}
		if next[len(next)-1] > hi {
			return p, "a cut as the first line starts"
		}
		start = next[len(next)-1]
	}

	endLo := offset + math.Min(tailMin, (post.To-offset)*0.5)
	endHi := math.Min(post.To-frameSec, offset+tailMax)
	if endLo > endHi {
		return p, "the pause after the last line is too short"
	}
	end := math.Min(math.Max(offset+tailIdeal, endLo), endHi)
	if c, ok := nearest(w.cutsIn(endLo-frameSec, endHi), offset+tailIdeal, cutSnapTail); ok {
		end = c
	}
	// a cut just before the end would show a flash of the next shot: end on it
	if cuts := w.cutsIn(end-closingShot, end-frameSec); len(cuts) > 0 && cuts[len(cuts)-1] >= endLo {
		end = cuts[len(cuts)-1]
	}

	if end-start < sceneMin {
		// more of the pause around the lines: after them first, then before
		end = math.Min(start+sceneMin, endHi)
		if end-start < sceneMin {
			start = math.Max(end-sceneMin, lo)
		}
	}
	if end-start > sceneMax {
		start = math.Max(start, onset-0.3)
		end = math.Min(end, offset+0.45)
	}
	start, end = frameDown(start), frameDown(end)
	if l := end - start; l < sceneMin-0.2 || l > sceneMax+frameSec || start > onset-0.08 || end < offset+0.15 {
		return p, fmt.Sprintf("no scene of 3 to 8 s fits (%.1f s)", l)
	}
	for _, b := range blocked {
		if b.overlaps(start, end) {
			return p, "lyrics"
		}
	}
	return scenePlace{Start: start, End: end, Onset: onset, Offset: offset}, ""
}

// nearest is the value closest to want, if it is within tolerance.
func nearest(values []float64, want, tolerance float64) (float64, bool) {
	best, found := 0.0, false
	for _, v := range values {
		if math.Abs(v-want) <= tolerance && (!found || math.Abs(v-want) < math.Abs(best-want)) {
			best, found = v, true
		}
	}
	return best, found
}

// frameDown rounds a time down to a whole frame (25 fps).
func frameDown(t float64) float64 { return math.Floor(t*FPS+1e-6) / FPS }
