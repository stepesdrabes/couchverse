package couchtv

import (
	"math"
	"slices"
)

// What the sound says of a scene (D43), for titles with no subtitles to read and as a check on
// those that have: where speech is, how clearly it is heard, whether the subtitles are in step.

// speechRun is utterances close enough to be one exchange, a candidate when nothing says what
// is spoken.
type speechRun struct {
	Utts []utterance
	// Score is 0 to 1, from the sound alone.
	Score float64
}

func (r speechRun) start() float64 { return r.Utts[0].From }
func (r speechRun) end() float64   { return r.Utts[len(r.Utts)-1].To }

const (
	// an utterance shorter than this is a breath or a click, longer than uttMax a speech
	uttMin, uttMax = 0.4, 6.0
	// utterances closer than this answer each other
	uttGap = 0.9
)

// speechRuns lists the exchanges of a stretch of audio (its first frame at origin) that a scene
// could play: one to four utterances bounded by pauses, with 1.2 to speechMax seconds of
// speech, scored from the sound alone.
func speechRuns(a vadAnalysis, origin float64) []speechRun {
	var utts []utterance
	for _, u := range a.utterances(vadMinPause) {
		if d := u.To - u.From; d >= uttMin && d <= uttMax {
			utts = append(utts, utterance{From: origin + u.From, To: origin + u.To})
		}
	}
	var out []speechRun
	for i := range utts {
		for j := i; j < len(utts) && j < i+4; j++ {
			if j > i && utts[j].From-utts[j-1].To >= uttGap {
				break
			}
			if utts[j].To-utts[i].From > speechMax {
				break
			}
			r := speechRun{Utts: utts[i : j+1]}
			if d := r.end() - r.start(); d < 1.2 {
				continue
			}
			r.Score = scoreSpeech(a, origin, r)
			out = append(out, r)
		}
	}
	return out
}

// scoreSpeech rates an exchange by how it sounds: a length a trailer line has, a back and
// forth rather than a drone, speech that is clearly heard over what is behind it, and
// expression (a voice that swells and drops, not a monotone), and pauses around it to cut in.
func scoreSpeech(a vadAnalysis, origin float64, r speechRun) float64 {
	span := r.end() - r.start()
	shape := 1.0
	switch {
	case span < 2.0:
		shape = 0.4 + 0.6*(span-1.2)/0.8
	case span > 5.2:
		shape = 1 - (span-5.2)/1.2*0.3
	}
	dialogue := 0.35
	switch len(r.Utts) {
	case 2:
		dialogue = 0.85
	case 3, 4:
		dialogue = 1
	}
	clear, _, ok := clarityOf(a, origin, r.start(), r.end())
	if !ok {
		return 0
	}
	expression := expressionOf(a, origin, r.start(), r.end())
	// pauses around: at least half a second to either side
	before, after := pauseAround(a, origin, r.start(), r.end())
	isolation := math.Min(1, math.Min(before, after)/0.6)
	return 0.30*shape + 0.25*dialogue + 0.25*clear + 0.12*expression + 0.08*isolation
}

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

// expressionOf is how much the voice's level moves within [from, to): the standard deviation
// of the level over the speech frames, 0 to 1 (a monotone is a drone; speech moves 5 to 9 dB).
func expressionOf(a vadAnalysis, origin, from, to float64) float64 {
	lo, hi := max(0, int((from-origin)/frameSec)), min(len(a.Speech), int((to-origin)/frameSec))
	var sum, sq float64
	n := 0
	for i := lo; i < hi; i++ {
		if a.Speech[i] {
			sum += a.Level[i]
			sq += a.Level[i] * a.Level[i]
			n++
		}
	}
	if n < 10 {
		return 0
	}
	mean := sum / float64(n)
	return math.Min(1, math.Sqrt(math.Max(0, sq/float64(n)-mean*mean))/9)
}

// pauseAround is how long it is quiet before [from, to) and after it (seconds, at most 1.5).
func pauseAround(a vadAnalysis, origin, from, to float64) (before, after float64) {
	i := int((from - origin) / frameSec)
	for before = 0; before < 1.5 && i-1-int(before/frameSec) >= 0 && !a.Active[i-1-int(before/frameSec)]; before += frameSec {
	}
	j := int((to - origin) / frameSec)
	for after = 0; after < 1.5 && j+int(after/frameSec) < len(a.Active) && !a.Active[j+int(after/frameSec)]; after += frameSec {
	}
	return before, after
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
