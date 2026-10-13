package couchtv

import "math"

// A break's mix is normalized here the way the render page normalizes a render
// (clients/broadcast/src/audio/loudness.ts, ported operation for operation, so both reach
// the same gain): ITU-R BS.1770-4 integrated loudness (K-weighting, 400 ms blocks with 75 %
// overlap, absolute and relative gates) and a true peak estimated by 4x oversampling. It
// measures as the samples stream through, so a long break never has to fit in memory.

// biquad is one K-weighting stage, its state kept between calls the way loudness.ts keeps
// it within one (inputs as the float32 samples they are, the recursion in float64).
type biquad struct {
	b, a           [3]float64
	x1, x2, y1, y2 float64
}

// step filters one sample; like loudness.ts, its output is stored as float32 before the
// next stage reads it.
func (q *biquad) step(x float32) float32 {
	v := q.b[0]*float64(x) + q.b[1]*q.x1 + q.b[2]*q.x2 - q.a[1]*q.y1 - q.a[2]*q.y2
	q.x2, q.x1 = q.x1, float64(x)
	q.y2, q.y1 = q.y1, v
	return float32(v)
}

// kWeighting is the shelf and the high-pass of BS.1770 for a sample rate (the libebur128
// derivation, as loudness.ts has it).
func kWeighting(rate float64) (shelf, highpass biquad) {
	f0 := 1681.974450955533
	g := 3.999843853973347
	q := 0.7071752369554196
	k := math.Tan(math.Pi * f0 / rate)
	vh := math.Pow(10, g/20)
	vb := math.Pow(vh, 0.4996667741545416)
	a0 := 1 + k/q + k*k
	shelf.b = [3]float64{(vh + vb*k/q + k*k) / a0, 2 * (k*k - vh) / a0, (vh - vb*k/q + k*k) / a0}
	shelf.a = [3]float64{1, 2 * (k*k - 1) / a0, (1 - k/q + k*k) / a0}
	f0 = 38.13547087602444
	q = 0.5003270373238773
	k = math.Tan(math.Pi * f0 / rate)
	a0 = 1 + k/q + k*k
	highpass.b = [3]float64{1, -2, 1}
	highpass.a = [3]float64{1, 2 * (k*k - 1) / a0, (1 - k/q + k*k) / a0}
	return shelf, highpass
}

// The 4x interpolator of the true-peak estimate: a Hann-windowed sinc, 12 taps per phase.
const (
	tpPhases = 4
	tpTaps   = 12
)

var tpFIR = func() (fir [tpPhases][tpTaps]float64) {
	for p := range tpPhases {
		for k := range tpTaps {
			n := float64(k*tpPhases + p)
			center := float64(tpTaps*tpPhases-1) / 2
			x := (n - center) / tpPhases
			sinc := 1.0
			if x != 0 {
				sinc = math.Sin(math.Pi*x) / (math.Pi * x)
			}
			w := 0.5 - 0.5*math.Cos(2*math.Pi*(n+0.5)/(tpTaps*tpPhases))
			fir[p][k] = sinc * w
		}
	}
	return fir
}()

// meter measures interleaved stereo float samples as they stream past.
type meter struct {
	shelf, highpass [2]biquad
	// block and step are a gating block (400 ms) and its hop (100 ms) in samples; the
	// squared K-weighted samples are summed per channel and hop
	block, step int
	hops        [][2]float64
	hop         [2]float64
	inHop       int
	// peak is the largest sample or interpolated value; recent holds each channel's last
	// tpTaps samples twice over, so they are always in one run ending at n%tpTaps+tpTaps;
	// n counts the samples each channel has seen
	peak   float64
	recent [2][2 * tpTaps]float32
	n      int
}

func newMeter(rate int) *meter {
	shelf, hp := kWeighting(float64(rate))
	return &meter{shelf: [2]biquad{shelf, shelf}, highpass: [2]biquad{hp, hp},
		block: int(math.Round(float64(rate) * 0.4)), step: int(math.Round(float64(rate) * 0.1))}
}

// add measures interleaved stereo samples.
func (m *meter) add(samples []float32) {
	for i := 0; i+1 < len(samples); i += 2 {
		for c := range 2 {
			x := samples[i+c]
			w := float64(m.highpass[c].step(m.shelf[c].step(x)))
			m.hop[c] += w * w
			m.see(math.Abs(float64(x)))
			r := &m.recent[c]
			at := m.n % tpTaps
			r[at], r[at+tpTaps] = x, x
			// loudness.ts interpolates from the sample after the first tpTaps, newest first
			if m.n >= tpTaps {
				for p := 1; p < tpPhases; p++ {
					s := 0.0
					for k := range tpTaps {
						s += tpFIR[p][k] * float64(r[at+tpTaps-k])
					}
					m.see(math.Abs(s))
				}
			}
		}
		m.n++
		if m.inHop++; m.inHop == m.step {
			m.hops = append(m.hops, m.hop)
			m.hop, m.inHop = [2]float64{}, 0
		}
	}
}

// see raises the peak to v (a NaN never does, as in loudness.ts).
func (m *meter) see(v float64) {
	if v > m.peak {
		m.peak = v
	}
}

// integrated is the integrated loudness in LUFS, -Inf for silence.
func (m *meter) integrated() float64 {
	per := m.block / m.step
	var powers []float64
	for j := 0; j+per <= len(m.hops); j++ {
		p := 0.0
		for c := range 2 {
			sum := 0.0
			for _, h := range m.hops[j : j+per] {
				sum += h[c]
			}
			p += sum / float64(m.block)
		}
		powers = append(powers, p)
	}
	lufs := func(p float64) float64 { return -0.691 + 10*math.Log10(p) }
	mean := func(xs []float64) float64 {
		s := 0.0
		for _, x := range xs {
			s += x
		}
		return s / float64(len(xs))
	}
	var abs []float64
	for _, p := range powers {
		if lufs(p) > -70 {
			abs = append(abs, p)
		}
	}
	if len(abs) == 0 {
		return math.Inf(-1)
	}
	gate := lufs(mean(abs)) - 10
	var rel []float64
	for _, p := range abs {
		if lufs(p) > gate {
			rel = append(rel, p)
		}
	}
	if len(rel) == 0 {
		rel = abs
	}
	return lufs(mean(rel))
}

// truePeak is the true peak in dBTP.
func (m *meter) truePeak() float64 { return 20 * math.Log10(m.peak+1e-12) }

// LoudnessReport is a normalization: the loudness measured, the gain it took and the result.
type LoudnessReport struct {
	Measured, GainDB, Integrated, TruePeak float64
}

// normalization is the gain that brings what the meter measured to target LUFS without
// letting the true peak pass ceiling dBTP (a quieter break beats a clipped one).
func (m *meter) normalization(target, ceiling float64) LoudnessReport {
	measured, peak := m.integrated(), m.truePeak()
	finite := !math.IsInf(measured, 0) && !math.IsNaN(measured)
	gain := 0.0
	if finite {
		gain = target - measured
	}
	if peak+gain > ceiling {
		gain = ceiling - peak
	}
	r := LoudnessReport{Measured: measured, GainDB: gain, Integrated: measured, TruePeak: peak + gain}
	if finite {
		r.Integrated = measured + gain
	}
	return r
}
