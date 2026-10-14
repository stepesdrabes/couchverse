package couchtv

import (
	"math"
	"slices"
)

// What the sound says of a scene (D43), as a check on the subtitles' scenes: how clearly the speech
// is heard, how clean the cuts are, whether the subtitles are in step.

// buriedBelow is how far (dB) the louder half of a line's frames must stand over the sound
// around it for the line to be heard.
const buriedBelow = 9.0

// clarityOf rates how clearly the speech in [from, to) is heard, 0 to 1: how far its louder half
// stands over the sound around it (the syllables, not the breaths between), and how much of its
// energy is in the speech band (music and effects have much outside it). over is that distance in
// dB; ok is false for speech too buried to use.
func clarityOf(a vadAnalysis, origin, from, to float64) (score, over float64, ok bool) {
	lo, hi := max(0, int((from-origin)/frameSec)), min(len(a.Speech), int((to-origin)/frameSec))
	var stands []float64
	var band float64
	for i := lo; i < hi; i++ {
		if a.Active[i] {
			stands = append(stands, a.Level[i]-a.Floor[i])
			band += a.Clarity[i]
		}
	}
	if len(stands) < 10 {
		return 0, 0, false
	}
	band /= float64(len(stands))
	slices.Sort(stands)
	for _, v := range stands[len(stands)/2:] {
		over += v
	}
	over /= float64(len(stands) - len(stands)/2)
	if over < buriedBelow {
		return 0, over, false
	}
	return 0.6*math.Min(1, (over-buriedBelow)/16) + 0.4*math.Min(1, band/0.55), over, true
}

// cutFit rates how clean a scene's two cuts are, 0.7 to 1: how far (dB) the sound in the first and
// last tenth of a second is below its speech. A pause the music or the noise goes on through is no
// cleaner a place to cut than a word; 12 dB or more is a clean cut, none costs 30 % of the score.
func cutFit(a vadAnalysis, origin float64, p scenePlace) float64 {
	level := func(from, to float64) float64 {
		lo, hi := max(0, int((from-origin)/frameSec)), min(len(a.Level), int((to-origin)/frameSec))
		if hi <= lo {
			return 0
		}
		var sum float64
		for _, v := range a.Level[lo:hi] {
			sum += v
		}
		return sum / float64(hi-lo)
	}
	var speech, n float64
	lo, hi := max(0, int((p.Onset-origin)/frameSec)), min(len(a.Level), int((p.Offset-origin)/frameSec))
	for i := lo; i < hi; i++ {
		if a.Speech[i] {
			speech += a.Level[i]
			n++
		}
	}
	if n < 10 {
		return 1
	}
	margin := speech/n - math.Max(level(p.Start, p.Start+0.1), level(p.End-0.1, p.End))
	return 0.7 + 0.3*math.Max(0, math.Min(margin, 12))/12
}

// syncReport counts how the speech a detector hears falls on subtitle cues: the frames inside
// cues and how many of them are speech, the frames away from every cue (by half a second) and
// how many of those are.
type syncReport struct {
	in, inN, out, outN int
}

func (s *syncReport) add(o syncReport) {
	s.in, s.inN, s.out, s.outN = s.in+o.in, s.inN+o.inN, s.out+o.out, s.outN+o.outN
}

// Inside is the share of the frames inside cues that are speech, Outside of the others.
func (s syncReport) Inside() float64  { return share(s.in, s.inN) }
func (s syncReport) Outside() float64 { return share(s.out, s.outN) }

func share(n, of int) float64 {
	if of == 0 {
		return 0
	}
	return float64(n) / float64(of)
}

// Ratio is how many times likelier speech is inside a cue than away from them.
func (s syncReport) Ratio() float64 { return s.Inside() / math.Max(s.Outside(), 0.02) }

// syncOf compares the speech a stretch of audio (its first frame at origin) holds with the
// cues over it, as if they were offset seconds later.
func syncOf(a vadAnalysis, origin float64, cues []dialogueCue, offset float64) syncReport {
	n := len(a.Speech)
	near := make([]bool, n)
	inside := make([]bool, n)
	mark := func(mask []bool, from, to float64) {
		for i := max(0, int((from-origin)/frameSec)); i < min(n, int((to-origin)/frameSec)); i++ {
			mask[i] = true
		}
	}
	for _, c := range cues {
		mark(inside, c.Start+offset, c.End+offset)
		mark(near, c.Start+offset-0.5, c.End+offset+0.5)
	}
	var r syncReport
	for i := range n {
		switch {
		case inside[i]:
			r.inN++
			if a.Speech[i] {
				r.in++
			}
		case !near[i]:
			r.outN++
			if a.Speech[i] {
				r.out++
			}
		}
	}
	return r
}

// pauseWindow is a stretch of audio that was decoded and the pauses in it.
type pauseWindow struct {
	from, to float64
	pauses   []pause
}

// Where an exchange's edges must fall to count as on a pause, in seconds: the first cue starts
// where a pause ends, the last cue ends where one starts.
const (
	onsetTolerance = 0.35
	fallTolerance  = 0.5
)

// edgeSync counts, for each offset, how many of the exchanges the windows cover have both their
// edges on a pause when the cues are that many seconds later. Speech is hard to tell from
// music, but a conversation starts after a pause and ends before one, and a track that is a
// few seconds off hits that three or four times less often than one in step.
func edgeSync(offsets []float64, wins []pauseWindow, exchanges []timeSpan) (hits, total []int) {
	hits, total = make([]int, len(offsets)), make([]int, len(offsets))
	for k, o := range offsets {
		for _, c := range exchanges {
			from, to := c.From+o, c.To+o
			var pauses []pause
			for _, w := range wins {
				if from-1 >= w.from && to+1 <= w.to {
					pauses = w.pauses
					break
				}
			}
			if pauses == nil {
				continue
			}
			total[k]++
			onset, fall := false, false
			for _, p := range pauses {
				onset = onset || !p.openRight && math.Abs(p.To-from) <= onsetTolerance
				fall = fall || !p.openLeft && math.Abs(p.From-to) <= fallTolerance
			}
			if onset && fall {
				hits[k]++
			}
		}
	}
	return hits, total
}
