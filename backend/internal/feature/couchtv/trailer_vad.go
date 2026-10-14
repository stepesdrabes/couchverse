package couchtv

import (
	"math"
	"slices"
)

// A voice-activity detector for the trailer picker: where in a stretch of mono 16 kHz audio
// someone speaks. It is cheap on purpose (the audio of half a film is decoded on a Pi): the
// energy in the speech band, 300 to 3400 Hz, in 20 ms frames against the noise floor around
// it, kept only where the energy rises and falls the way syllables do (a few swells a second),
// which loud steady music and effects do not. It is used to check subtitles against the
// sound, to place a scene's edges in pauses, and to find speech where there are no subtitles.

const (
	vadRate  = 16000
	vadFrame = vadRate / 50 // 20 ms
	// frameSec is the length of one frame.
	frameSec = 1.0 / 50
)

// The detector's fixed settings, in frames (20 ms) and dB.
const (
	vadAbsFloor  = -62 // dB: below this it is silence whatever the floor
	vadFloorSpan = 150 // the noise floor is the 10th percentile of three seconds around
	vadSmooth    = 12  // the envelope's slow part is its mean over +-0.24 s
	vadModWindow = 15  // syllabic modulation is judged over +-0.3 s
	vadHangover  = 5   // a speech frame carries on for 0.1 s
	vadShortest  = 4   // speech shorter than this (80 ms) is a click
	vadFillGap   = 6   // a gap inside speech shorter than this (120 ms) is not a pause
	vadGrow      = 25  // a weak frame counts within this many of a sure one
	vadQuiet     = 5.5 // a frame this near the noise floor is quiet
	// utterances are bounded by pauses of at least this many seconds
	vadMinPause = 0.28
)

// vadProfile says how sure a frame must be. A sure speech frame stands `above` dB over the
// noise floor in an envelope that swells `depth` dB rms at `rate` swells a second; the quiet
// starts and tails of words next to it need only the weak figures. Finding speech wants few
// false alarms (music is not a line), finding pauses wants few misses (a whisper is not a
// pause): two profiles.
type vadProfile struct {
	above, depth, rate             float64
	weakAbove, weakDepth, weakRate float64
}

var (
	speechProfile = vadProfile{above: 8, depth: 3.2, rate: 2, weakAbove: 4, weakDepth: 1.5, weakRate: 1}
	activeProfile = vadProfile{above: 5, depth: 2, rate: 1.5, weakAbove: 3, weakDepth: 1, weakRate: 1}
)

// vadAnalysis is what the detector found in a stretch of audio.
type vadAnalysis struct {
	// Speech says per 20 ms frame whether someone speaks (few false alarms); Active whether
	// anything might be speech, so a frame that is not Active is a pause.
	Speech, Active []bool
	// Quiet marks the frames where nothing stands out over the background: the pauses.
	Quiet []bool
	// Level is the frame's band energy in dB, Floor the noise floor under it.
	Level, Floor []float64
	// Clarity is the band energy over the whole signal's energy, per frame (0 to 1).
	Clarity []float64
}

// speechMeter measures mono 16 kHz audio frame by frame as it streams in, so half a film's
// audio never has to be held at once: only each frame's two energies are kept.
type speechMeter struct {
	hp, hp2, lp, lp2 *voiceFilter
	band, full       []float64
	accBand, accFull float64
	filled           int
}

func newSpeechMeter() *speechMeter {
	return &speechMeter{hp: newVoiceFilter(voiceHigh, 300, vadRate), hp2: newVoiceFilter(voiceHigh, 300, vadRate),
		lp: newVoiceFilter(voiceLow, 3400, vadRate), lp2: newVoiceFilter(voiceLow, 3400, vadRate)}
}

// add measures more samples; a frame is complete every vadFrame of them.
func (m *speechMeter) add(x []float32) {
	for _, s := range x {
		v := float64(s)
		y := m.lp2.step(m.lp.step(m.hp2.step(m.hp.step(v))))
		m.accBand += y * y
		m.accFull += v * v
		if m.filled++; m.filled == vadFrame {
			m.band = append(m.band, m.accBand/vadFrame)
			m.full = append(m.full, m.accFull/vadFrame)
			m.accBand, m.accFull, m.filled = 0, 0, 0
		}
	}
}

// analyzeSpeech runs the detector over mono 16 kHz samples.
func analyzeSpeech(x []float32) vadAnalysis {
	m := newSpeechMeter()
	m.add(x)
	return m.analysis()
}

// analysis runs the detector over the frames measured so far.
func (m *speechMeter) analysis() vadAnalysis {
	band, full := m.band, m.full
	n := len(band)
	a := vadAnalysis{Level: make([]float64, n), Floor: make([]float64, n), Clarity: make([]float64, n)}
	for i := range n {
		a.Level[i] = 10 * math.Log10(band[i]+1e-12)
		a.Clarity[i] = band[i] / (full[i] + 1e-12)
	}
	a.Floor = noiseFloor(a.Level)

	// the envelope's fast part: what is left of the level when its slow mean is taken away
	slow := movingMean(a.Level, vadSmooth)
	fast := make([]float64, n)
	for i := range fast {
		fast[i] = a.Level[i] - slow[i]
	}
	depth, rate := make([]float64, n), make([]float64, n)
	for i := range n {
		lo, hi := max(0, i-vadModWindow), min(n, i+vadModWindow+1)
		var sum float64
		crossings := 0
		for k := lo; k < hi; k++ {
			sum += fast[k] * fast[k]
			if k > lo && (fast[k] >= 0) != (fast[k-1] >= 0) {
				crossings++
			}
		}
		depth[i] = math.Sqrt(sum / float64(hi-lo))
		rate[i] = float64(crossings) / (float64(hi-lo) * frameSec) / 2 // swells a second
	}
	a.Quiet = make([]bool, n)
	for i := range n {
		a.Quiet[i] = a.Level[i] < a.Floor[i]+vadQuiet
	}
	a.Speech = a.detect(speechProfile, depth, rate)
	a.Active = a.detect(activeProfile, depth, rate)
	return a
}

// detect marks the frames a profile takes for speech.
func (a vadAnalysis) detect(p vadProfile, depth, rate []float64) []bool {
	n := len(a.Level)
	strong, weak := make([]bool, n), make([]bool, n)
	for i := range n {
		if a.Level[i] < vadAbsFloor {
			continue
		}
		over := a.Level[i] - a.Floor[i]
		weak[i] = over >= p.weakAbove && depth[i] >= p.weakDepth && rate[i] >= p.weakRate
		strong[i] = over >= p.above && depth[i] >= p.depth && rate[i] >= p.rate
	}
	return tidySpeech(growSpeech(strong, weak, vadGrow))
}

// growSpeech extends the sure frames over the weak ones next to them, at most grow frames.
func growSpeech(strong, weak []bool, grow int) []bool {
	out := slices.Clone(strong)
	for i, s := range strong {
		if !s {
			continue
		}
		for k := i - 1; k >= max(0, i-grow) && weak[k] && !out[k]; k-- {
			out[k] = true
		}
		for k := i + 1; k <= min(len(out)-1, i+grow) && weak[k] && !out[k]; k++ {
			out[k] = true
		}
	}
	return out
}

// noiseFloor is the 10th percentile of the level over the frames around each one, measured per
// half-window block and then smoothed so that a pause between lines does not become the floor
// for the line after it.
func noiseFloor(level []float64) []float64 {
	n := len(level)
	floor := make([]float64, n)
	half := vadFloorSpan / 2
	blocks := (n + half - 1) / half
	p10 := make([]float64, blocks)
	for b := range blocks {
		seg := slices.Clone(level[b*half : min(n, (b+1)*half)])
		slices.Sort(seg)
		p10[b] = seg[len(seg)/10]
	}
	for i := range floor {
		b := i / half
		floor[i] = p10[b]
		for _, k := range []int{b - 1, b + 1} {
			if k >= 0 && k < blocks {
				floor[i] = math.Min(floor[i], p10[k]+4) // a neighbouring block's floor, a little looser
			}
		}
	}
	return floor
}

func movingMean(x []float64, radius int) []float64 {
	out := make([]float64, len(x))
	prefix := make([]float64, len(x)+1)
	for i, v := range x {
		prefix[i+1] = prefix[i] + v
	}
	for i := range x {
		lo, hi := max(0, i-radius), min(len(x), i+radius+1)
		out[i] = (prefix[hi] - prefix[lo]) / float64(hi-lo)
	}
	return out
}

// tidySpeech drops clicks, carries speech over its hangover and joins stretches a short
// pause apart.
func tidySpeech(raw []bool) []bool {
	n := len(raw)
	out := slices.Clone(raw)
	for i := 0; i < n; {
		if !out[i] {
			i++
			continue
		}
		j := i
		for j < n && out[j] {
			j++
		}
		if j-i < vadShortest {
			for k := i; k < j; k++ {
				out[k] = false
			}
		}
		i = j
	}
	// hangover, then fill short gaps
	last := -1000
	held := make([]bool, n)
	for i := range n {
		if out[i] {
			last = i
		}
		held[i] = out[i] || i-last <= vadHangover
	}
	for i := 0; i < n; {
		if held[i] {
			i++
			continue
		}
		j := i
		for j < n && !held[j] {
			j++
		}
		if i > 0 && j < n && j-i < vadFillGap {
			for k := i; k < j; k++ {
				held[k] = true
			}
		}
		i = j
	}
	return held
}

// voiceFilter is a second-order filter (RBJ cookbook, Q of a Butterworth).
type voiceFilter struct {
	b0, b1, b2, a1, a2 float64
	x1, x2, y1, y2     float64
}

const (
	voiceHigh = iota
	voiceLow
)

func newVoiceFilter(kind int, freq, rate float64) *voiceFilter {
	w := 2 * math.Pi * freq / rate
	alpha := math.Sin(w) / math.Sqrt2 // Q = 1/sqrt(2)
	cw := math.Cos(w)
	var b0, b1, b2 float64
	if kind == voiceHigh {
		b0, b1, b2 = (1+cw)/2, -(1 + cw), (1+cw)/2
	} else {
		b0, b1, b2 = (1-cw)/2, 1-cw, (1-cw)/2
	}
	a0 := 1 + alpha
	return &voiceFilter{b0: b0 / a0, b1: b1 / a0, b2: b2 / a0, a1: -2 * cw / a0, a2: (1 - alpha) / a0}
}

func (f *voiceFilter) step(x float64) float64 {
	y := f.b0*x + f.b1*f.x1 + f.b2*f.x2 - f.a1*f.y1 - f.a2*f.y2
	f.x2, f.x1, f.y2, f.y1 = f.x1, x, f.y1, y
	return y
}

// utterance is a stretch of speech between pauses, in seconds from the audio's start.
type utterance = timeSpan

// utterances lists the speech of an analysis bounded by pauses of at least minPause seconds.
func (a vadAnalysis) utterances(minPause float64) []utterance {
	var out []utterance
	n := len(a.Speech)
	gap := int(math.Ceil(minPause / frameSec))
	for i := 0; i < n; {
		if !a.Speech[i] {
			i++
			continue
		}
		j := i
		for j < n && a.Speech[j] {
			j++
		}
		u := utterance{From: float64(i) * frameSec, To: float64(j) * frameSec}
		if len(out) > 0 && (i-int(math.Round(out[len(out)-1].To/frameSec))) < gap {
			out[len(out)-1].To = u.To
		} else {
			out = append(out, u)
		}
		i = j
	}
	return out
}

// speechIn is the fraction of the frames in [from, to) seconds that are speech.
func (a vadAnalysis) speechIn(from, to float64) float64 {
	lo, hi := max(0, int(from/frameSec)), min(len(a.Speech), int(math.Ceil(to/frameSec)))
	if hi <= lo {
		return 0
	}
	n := 0
	for _, s := range a.Speech[lo:hi] {
		if s {
			n++
		}
	}
	return float64(n) / float64(hi-lo)
}
