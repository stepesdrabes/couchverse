package couchtv

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
)

// A break's sound is its items' sounds mixed (plan 6.7). Each item, a clip's sound as one
// render of the whole break used to play it (from its start, faded out over the crossfade
// where the next item starts, ringing out under it), is rendered once by the page
// (`renderItemAudio`) at the engine's own level and kept in couchtv/sounds/<key>.wav, a
// 48 kHz stereo float WAV, for every break that has it; a clip's render makes its own.
// Assembling a break adds them up at their starts and normalizes the sum as the page
// normalizes a render.

const (
	soundsDir = "sounds"
	// sampleRate is the page's audio rate.
	sampleRate = 48000
	// mixChunk is how much of a break is mixed at a time, so a long break never has to fit
	// in memory.
	mixChunk = 10 * sampleRate
	// truePeakCeiling is where the page caps a render's true peak (dBTP).
	truePeakCeiling = -1.0
)

// bodyAiring stands in for a promo's airing in its sound's key, as the promo body renders
// with a fixed airing (promo.ts): the words are seen, never heard, so a promo sounds the same
// whenever it airs, but with an airing or without one (the evergreen end) it does not.
var bodyAiring = SpecAiring{Eyebrow: "Tonight", Time: "20:15"}

// itemSpec is the spec whose sound a clip of the given kind plays in a break, false for a
// clip that has none of its own there: a promo body's sound is the full promo, which rings
// on through its end card; the end cards and the standby card make none.
func itemSpec(kind string, spec Spec) (Spec, bool) {
	switch kind {
	case ClipPromoEnd, ClipEvergreenEnd, ClipStandby:
		return Spec{}, false
	case ClipPromoBody:
		spec.Part = "full"
		airing := bodyAiring
		spec.Airing = &airing
	}
	return spec, true
}

// soundSpec is the spec an item's sound is rendered from and keyed by: a promo's airing is
// replaced by the body's (see bodyAiring).
func soundSpec(spec Spec) Spec {
	if spec.Template == "promo" && spec.Airing != nil {
		airing := bodyAiring
		spec.Airing = &airing
	}
	return spec
}

// soundKey is the cache key of an item's sound: its template's version and its sound spec,
// like a clip's key (equal keys sound the same).
func soundKey(version int, spec Spec) (string, error) {
	data, err := json.Marshal(soundSpec(spec))
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(fmt.Appendf(nil, "sound:%s:%d:%s", spec.Template, version, data))
	return hex.EncodeToString(sum[:16]), nil
}

func soundPath(dataDir, key string) string {
	return filepath.Join(dataDir, "couchtv", soundsDir, key+".wav")
}

// wavHeader is the 44-byte header the page writes before its float samples.
const wavHeader = 44

// wavSamples is how many stereo samples a sound the page made holds, checking that it is its
// 48 kHz stereo float WAV.
func wavSamples(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	var head [wavHeader]byte
	if _, err := io.ReadFull(f, head[:]); err != nil {
		return 0, err
	}
	if string(head[36:40]) != "data" || binary.LittleEndian.Uint16(head[20:]) != 3 ||
		binary.LittleEndian.Uint16(head[22:]) != 2 || binary.LittleEndian.Uint32(head[24:]) != sampleRate {
		return 0, errors.New("not a 48 kHz stereo float WAV")
	}
	info, err := f.Stat()
	if err != nil {
		return 0, err
	}
	n := int64(binary.LittleEndian.Uint32(head[40:]))
	if info.Size() < wavHeader+n {
		return 0, errors.New("the WAV is cut short")
	}
	return int(n / 8), nil
}

// mixItem is one item's sound in a break.
type mixItem struct {
	path string
	// at is where the item starts in the break and slot how long until the next item
	// starts, in samples; its sound must last at least that long
	at, slot int
}

// mixBreak mixes items into out, interleaved 32-bit float stereo exactly samples long: every
// item's sound added at its start (its ring-out under the items after it, whatever runs
// past the break's end cut), the sum normalized to loudness LUFS with the true peak at or
// under -1 dBTP, as the page normalizes a render.
func mixBreak(items []mixItem, samples int, loudness float64, out string) (LoudnessReport, error) {
	type source struct {
		mixItem
		f *os.File
		n int
	}
	srcs := make([]source, len(items))
	for i, it := range items {
		n, err := wavSamples(it.path)
		if err != nil {
			return LoudnessReport{}, fmt.Errorf("%s: %w", filepath.Base(it.path), err)
		}
		if n < it.slot {
			return LoudnessReport{}, fmt.Errorf("%s: %d samples, its slot is %d", filepath.Base(it.path), n, it.slot)
		}
		f, err := os.Open(it.path)
		if err != nil {
			return LoudnessReport{}, err
		}
		defer f.Close()
		srcs[i] = source{mixItem: it, f: f, n: n}
	}
	dst, err := os.OpenFile(out, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return LoudnessReport{}, err
	}
	defer dst.Close()

	m := newMeter(sampleRate)
	mix := make([]float32, 2*mixChunk)
	raw := make([]byte, 8*mixChunk)
	for from := 0; from < samples; from += mixChunk {
		n := min(mixChunk, samples-from)
		clear(mix[:2*n])
		for _, s := range srcs {
			// the part of the item's sound inside [from, from+n)
			lo, hi := max(from, s.at), min(from+n, s.at+s.n)
			if lo >= hi {
				continue
			}
			b := raw[:8*(hi-lo)]
			if _, err := s.f.ReadAt(b, int64(wavHeader+8*(lo-s.at))); err != nil {
				return LoudnessReport{}, err
			}
			for i, o := 0, 2*(lo-from); i < len(b); i, o = i+4, o+1 {
				mix[o] += math.Float32frombits(binary.LittleEndian.Uint32(b[i:]))
			}
		}
		m.add(mix[:2*n])
		if _, err := dst.Write(f32bytes(raw, mix[:2*n])); err != nil {
			return LoudnessReport{}, err
		}
	}

	report := m.normalization(loudness, truePeakCeiling)
	g := math.Pow(10, report.GainDB/20)
	for from := 0; from < samples; from += mixChunk {
		b := raw[:8*min(mixChunk, samples-from)]
		if _, err := dst.ReadAt(b, int64(8*from)); err != nil {
			return report, err
		}
		for i := 0; i < len(b); i += 4 {
			v := float32(float64(math.Float32frombits(binary.LittleEndian.Uint32(b[i:]))) * g)
			binary.LittleEndian.PutUint32(b[i:], math.Float32bits(v))
		}
		if _, err := dst.WriteAt(b, int64(8*from)); err != nil {
			return report, err
		}
	}
	return report, dst.Close()
}

// f32bytes writes samples into buf as little-endian float32 and returns the bytes written.
func f32bytes(buf []byte, samples []float32) []byte {
	for i, v := range samples {
		binary.LittleEndian.PutUint32(buf[4*i:], math.Float32bits(v))
	}
	return buf[:4*len(samples)]
}
