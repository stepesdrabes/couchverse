package couchtv

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// loudnessFixture is a signal the page's loudness.ts test builds bit for bit: integer noise
// and a pulse train in float32, loud for 4 s, 20 dB down for 2 s, silent for 2 s, then hot
// pulses whose peaks fall between samples.
func loudnessFixture(seconds int) []float32 {
	n := seconds * sampleRate
	out := make([]float32, 2*n)
	for c, seed := range []uint32{12345, 67890} {
		s := seed
		for i := range n {
			s = s*1664525 + 1013904223
			noise := float64(s>>9)/8388608 - 0.5
			level := 0.9
			switch t := i / sampleRate; {
			case t < 4:
				level = 0.5
			case t < 6:
				level = 0.05
			case t < 8:
				level = 0
			}
			pulse := -0.25
			if i%96 < 48 {
				pulse = 0.25
			}
			out[2*i+c] = float32((noise*0.6 + pulse) * level)
		}
	}
	return out
}

func TestLoudnessMatchesThePage(t *testing.T) {
	x := loudnessFixture(10)
	m := newMeter(sampleRate)
	// streamed in uneven pieces: the measure cannot depend on where they break
	for at := 0; at < len(x); {
		step := 2 * (999 + (at/2)%4801)
		m.add(x[at:min(len(x), at+step)])
		at += step
	}
	// what loudness.ts measures of the same samples (clients/broadcast/test/unit/audio.test.ts)
	near := func(what string, got, want float64) {
		t.Helper()
		if math.Abs(got-want) > 1e-9 {
			t.Errorf("%s %.12f, the page's %.12f", what, got, want)
		}
	}
	near("integrated", m.integrated(), -9.855156242089157)
	near("true peak", m.truePeak(), -2.852273586834342)
	r := m.normalization(-18, truePeakCeiling)
	near("gain to -18", r.GainDB, -8.144843757910843)
	near("loudness at -18", r.Integrated, -18)
	// hot enough that the true peak stops it at -1 dBTP
	r = m.normalization(-5, truePeakCeiling)
	near("gain to -5", r.GainDB, 1.8522735868343418)
	near("loudness capped", r.Integrated, -8.002882655254815)
	near("capped true peak", r.TruePeak, -1)

	if got := newMeter(sampleRate).integrated(); !math.IsInf(got, -1) {
		t.Errorf("silence measures %g", got)
	}
}

// writeItem writes interleaved stereo samples as the page writes an item's sound.
func writeItem(t *testing.T, path string, samples []float32) {
	t.Helper()
	head := make([]byte, wavHeader)
	copy(head, "RIFF")
	binary.LittleEndian.PutUint32(head[4:], uint32(36+4*len(samples)))
	copy(head[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(head[16:], 16)
	binary.LittleEndian.PutUint16(head[20:], 3)
	binary.LittleEndian.PutUint16(head[22:], 2)
	binary.LittleEndian.PutUint32(head[24:], sampleRate)
	binary.LittleEndian.PutUint32(head[28:], sampleRate*8)
	binary.LittleEndian.PutUint16(head[32:], 8)
	binary.LittleEndian.PutUint16(head[34:], 32)
	copy(head[36:], "data")
	binary.LittleEndian.PutUint32(head[40:], uint32(4*len(samples)))
	if err := os.WriteFile(path, append(head, f32bytes(make([]byte, 4*len(samples)), samples)...), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readF32(t *testing.T, path string) []float32 {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]float32, len(b)/4)
	for i := range out {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[4*i:]))
	}
	return out
}

// tone is n stereo samples of a sine at amp, the right channel at half of it.
func tone(n int, freq, amp float64) []float32 {
	out := make([]float32, 2*n)
	for i := range n {
		v := amp * math.Sin(2*math.Pi*freq*float64(i)/sampleRate)
		out[2*i], out[2*i+1] = float32(v), float32(v/2)
	}
	return out
}

func TestMixBreak(t *testing.T) {
	dir := t.TempDir()
	const second = sampleRate
	// a 2.5 s item ringing out 1 s past its 1.5 s slot, then a 12 s one (longer than one mix
	// chunk) whose ring-out runs past the break's end
	a, b := tone(5*second/2, 220, 0.3), tone(12*second, 440, 0.2)
	writeItem(t, filepath.Join(dir, "a.wav"), a)
	writeItem(t, filepath.Join(dir, "b.wav"), b)
	items := []mixItem{
		{path: filepath.Join(dir, "a.wav"), at: 0, slot: 3 * second / 2},
		{path: filepath.Join(dir, "b.wav"), at: 3 * second / 2, slot: 10 * second},
	}
	total := 23 * second / 2
	out := filepath.Join(dir, "mix.f32")
	r, err := mixBreak(items, total, -20, out)
	if err != nil {
		t.Fatal(err)
	}
	mix := readF32(t, out)
	if len(mix) != 2*total {
		t.Fatalf("%d samples, the break is %d", len(mix)/2, total)
	}
	g := math.Pow(10, r.GainDB/20)
	for i := range total {
		for c := range 2 {
			want := 0.0
			if i < len(a)/2 {
				want += float64(a[2*i+c])
			}
			if j := i - items[1].at; j >= 0 {
				want += float64(b[2*j+c])
			}
			if got := float64(mix[2*i+c]); math.Abs(got-want*g) > 1e-6 {
				t.Fatalf("sample %d.%d is %g, want %g", i, c, got, want*g)
			}
		}
	}
	m := newMeter(sampleRate)
	m.add(mix)
	if l := m.integrated(); math.Abs(l+20) > 0.001 || math.Abs(r.Integrated+20) > 1e-9 {
		t.Errorf("the mix measures %.4f LUFS (reported %.4f), want -20", l, r.Integrated)
	}
	if p := m.truePeak(); p > truePeakCeiling+1e-6 {
		t.Errorf("true peak %.3f dBTP", p)
	}

	// louder than the true peak allows: quieter, never clipped
	if r, err = mixBreak(items, total, -8, out); err != nil {
		t.Fatal(err)
	}
	m = newMeter(sampleRate)
	m.add(readF32(t, out))
	if p := m.truePeak(); math.Abs(p-truePeakCeiling) > 1e-4 || r.Integrated > -8.5 {
		t.Errorf("at -8 LUFS: %.4f dBTP, %.2f LUFS", p, r.Integrated)
	}

	// an item's sound must last its slot
	items[1].slot = 13 * second
	if _, err := mixBreak(items, total+3*second, -20, out); err == nil || !strings.Contains(err.Error(), "slot") {
		t.Errorf("a sound shorter than its slot mixed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.wav"), []byte("RIFF"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := mixBreak(items[:1], second, -20, out); err == nil {
		t.Error("a broken item mixed")
	}
}

func TestSoundKeys(t *testing.T) {
	title := specTitle("night")
	body := Spec{Template: "promo", Part: "body", Mood: "cinematic", Title: &title, Bed: &Bed{URL: "/media/v1/audio/f/0_20000.flac", Out: 20}}
	key := func(version int, s Spec) string {
		t.Helper()
		k, err := soundKey(version, s)
		if err != nil {
			t.Fatal(err)
		}
		return k
	}
	full := func(a *SpecAiring) Spec {
		s := body
		s.Part, s.Airing = "full", a
		return s
	}
	tonight := full(&SpecAiring{Eyebrow: "Tonight", Time: "20:15"})
	premiere := full(&SpecAiring{Eyebrow: "Premiere", Day: "Saturday", Time: "21:45"})
	evergreen := full(nil)

	// a body's own render is the sound of its full promo, whenever that airs
	item, ok := itemSpec(ClipPromoBody, body)
	if !ok || key(2, item) != key(2, tonight) || key(2, premiere) != key(2, tonight) {
		t.Error("a promo body's sound depends on its airing")
	}
	if key(2, evergreen) == key(2, tonight) {
		t.Error("the evergreen end card (no airing) shares the airing's sound")
	}
	if key(3, tonight) == key(2, tonight) {
		t.Error("a new template version keeps the old sound")
	}
	// other templates make sounds of what their airings say (a strip promo ticks its days)
	strip := Spec{Template: "stripPromo", Title: &title, Airing: &SpecAiring{Eyebrow: "Every night", Time: "18:00", Days: []int{0, 1, 2, 3, 4, 5, 6}}}
	weekdays := strip
	weekdays.Airing = &SpecAiring{Eyebrow: "Every weekday", Time: "18:00", Days: []int{0, 1, 2, 3, 4}}
	if key(1, strip) == key(1, weekdays) {
		t.Error("strip promos of different days share a sound")
	}
	if s, ok := itemSpec(ClipSting, Spec{Template: "sting", Duration: 3}); !ok || s.Duration != 3 {
		t.Error("a sting is not its own item")
	}
	for _, kind := range []string{ClipPromoEnd, ClipEvergreenEnd, ClipStandby} {
		if _, ok := itemSpec(kind, body); ok {
			t.Errorf("%s has a sound of its own in a break", kind)
		}
	}
}
