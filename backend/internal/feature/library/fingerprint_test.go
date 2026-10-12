package library

import (
	"fmt"
	"math"
	"math/rand/v2"
	"testing"

	"couchverse/internal/media"
)

// noise is a fingerprint of unrelated sound: random items.
func noise(r *rand.Rand, n int) []uint32 {
	out := make([]uint32, n)
	for i := range out {
		out[i] = r.Uint32()
	}
	return out
}

func TestMatchRunFindsTheSharedSegment(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	theme := noise(r, 250)
	a := append(append(noise(r, 100), theme...), noise(r, 300)...)
	b := append(append(noise(r, 400), theme...), noise(r, 50)...)
	// a few bits off everywhere, and one item lost entirely: still one run
	for i := 120; i < 350; i += 7 {
		a[i] ^= 0b10101
	}
	b[400+130] = ^b[400+130]

	got := matchRun(a, b)
	if got.n < 249 || got.a != 100 || got.b != 400 {
		t.Errorf("run %+v, want 250 items at 100/400", got)
	}

	// two lost items in a row end the run
	b[400+60], b[400+61] = ^b[400+60], ^b[400+61]
	if got := matchRun(a, b); got.n != 250-62 {
		t.Errorf("run %+v, want the part after the gap", got)
	}
}

func TestConsensus(t *testing.T) {
	for _, c := range []struct {
		name  string
		found []segment
		want  *segment
	}{
		{"two of three agree", []segment{{20.8, 50}, {20.7, 69}, {300, 320}}, &segment{20.8, 50}},
		{"starts differ, ends agree", []segment{{83.2, 115.8}, {79.3, 115.3}}, &segment{83.2, 115.3}},
		{"no two agree", []segment{{124.7, 232.7}, {187.4, 221.5}}, nil},
		{"one find is not a consensus", []segment{{10, 40}}, nil},
		{"too short", []segment{{10, 16}, {10.5, 16.2}}, nil},
		{"short finds do not vote", []segment{{10, 40}, {10.2, 15}}, nil},
		{"the shared part is too short", []segment{{10, 30}, {11, 18.5}}, nil},
		{"the larger group wins", []segment{{10, 40}, {11, 40}, {200, 260}, {200.5, 259}, {201, 260}}, &segment{201, 259}},
	} {
		got, ok := consensus(c.found)
		switch {
		case c.want == nil && ok:
			t.Errorf("%s: %+v, want none", c.name, got)
		case c.want != nil && (!ok || got != *c.want):
			t.Errorf("%s: %+v (%v), want %+v", c.name, got, ok, *c.want)
		}
	}
}

func TestPartners(t *testing.T) {
	eps := []string{"e1", "e2", "e3", "e3", "e4", "e5"}
	for _, c := range []struct {
		i    int
		want string
	}{
		{0, "[1 2 3]"},
		{2, "[1 4 0]"}, // e3's other file is not a partner
		{5, "[4 3 2]"},
	} {
		if got := fmt.Sprint(partners(eps, c.i)); got != c.want {
			t.Errorf("partners of %d: %s, want %s", c.i, got, c.want)
		}
	}
	if got := partners([]string{"e1"}, 0); len(got) != 0 {
		t.Errorf("a lone episode has partners %v", got)
	}
}

func loadFingerprint(t *testing.T, name string) []uint32 {
	t.Helper()
	fp, err := readFingerprint("testdata/fingerprints/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return fp
}

// TestSharedSegmentsOnProductionFingerprints runs the matching on fingerprints taken from
// the production library in the S3 spike (the first 8 and last 6 minutes of episodes
// 2-4 of a season), against what the spike found by hand.
func TestSharedSegmentsOnProductionFingerprints(t *testing.T) {
	type want struct{ start, length float64 } // seconds; length 0 for none
	for _, c := range []struct {
		name, pattern string
		offset        float64
		want          []want
	}{
		// the intro after a cold open of varying length; one pair loses its first seconds
		{"South Park S1 intro", "south-park-s01e%02d.head.fp", 0, []want{{106.6, 36}, {83.2, 32.1}, {108.7, 32.1}}},
		// Invincible's 9-14 s title sting, once nearly 8 minutes in
		{"Invincible S1 sting", "invincible-s01e%02d.head.fp", 0, []want{{3.2, 9.3}, {467, 9.3}, {3.7, 10}}},
		// files whose opening was cut: no intro
		{"Simpsons S18", "simpsons-s18e%02d.head.fp", 0, []want{{}, {}, {}}},
		// end credits, from 6 minutes before the end
		{"South Park S13 credits", "south-park-s13e%02d.tail.fp", 1000, []want{{1330.8, 26.6}, {1330.8, 26.6}, {1330.8, 26.6}}},
	} {
		var fps [][]uint32
		var offsets []float64
		for e := 2; e <= 4; e++ {
			fps = append(fps, loadFingerprint(t, fmt.Sprintf(c.pattern, e)))
			offsets = append(offsets, c.offset)
		}
		got := sharedSegments(fps, offsets, []string{"e2", "e3", "e4"})
		for i, w := range c.want {
			s := got[i]
			switch {
			case w.length == 0 && s != nil:
				t.Errorf("%s e%d: %+v, want none", c.name, i+2, *s)
			case w.length > 0 && s == nil:
				t.Errorf("%s e%d: none, want %.1f s at %.1f", c.name, i+2, w.length, w.start)
			case w.length > 0 && (math.Abs(s.start-w.start) > 0.2 || math.Abs(s.length()-w.length) > 0.2):
				t.Errorf("%s e%d: %.1f s at %.1f, want %.1f s at %.1f", c.name, i+2, s.length(), s.start, w.length, w.start)
			}
		}
	}
}

func TestWindows(t *testing.T) {
	headFrom, headLen, tailFrom, tailLen := windows(1320)
	if headFrom != 0 || headLen != headWindow || tailFrom != 960 || tailLen != tailWindow {
		t.Errorf("a 22-minute episode: %v %v %v %v", headFrom, headLen, tailFrom, tailLen)
	}
	headFrom, headLen, tailFrom, tailLen = windows(200)
	if headFrom != 0 || headLen != 200 || tailFrom != 0 || tailLen != 200 {
		t.Errorf("a short file: %v %v %v %v", headFrom, headLen, tailFrom, tailLen)
	}
	if math.Abs(media.FingerprintItem-0.1238) > 0.0001 {
		t.Errorf("an item spans %v s", media.FingerprintItem)
	}
}
