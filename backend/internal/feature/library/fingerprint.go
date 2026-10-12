package library

import (
	"math"
	"math/bits"

	"couchverse/internal/media"
)

// Theme detection (docs/couch-tv-plan.md 3.1): two episodes of a season share their
// intro (and their end credits), so their chromaprints agree item for item at some
// offset. The longest such run is the shared segment.
const (
	// matchMaxBits is how many of an item's 32 bits may differ for it to match.
	matchMaxBits = 10
	// matchMinOverlap is the shortest overlap (items) an offset is tried at.
	matchMinOverlap = 40
	// minSegment is the shortest shared segment that counts (Invincible's title sting).
	minSegment = 8.0
	// agreeWithin is how close (seconds) pairs must place a segment to agree on it.
	agreeWithin = 2.0
	// pairsPerFile is how many other episodes each one is matched with.
	pairsPerFile = 3
)

// run is the longest stretch two fingerprints share: n items from a[a] and b[b].
type run struct{ n, a, b int }

// matchRun finds the longest run of matching items over every offset between a and b.
// A single item that misses between two that match counts as matching (a dropout).
func matchRun(a, b []uint32) run {
	var best run
	ok := make([]bool, min(len(a), len(b)))
	for shift := -len(b) + matchMinOverlap; shift < len(a)-matchMinOverlap; shift++ {
		a0, b0 := max(shift, 0), max(-shift, 0)
		n := min(len(a)-a0, len(b)-b0)
		if n <= best.n {
			continue
		}
		for i := range n {
			ok[i] = bits.OnesCount32(a[a0+i]^b[b0+i]) <= matchMaxBits
		}
		length, start := 0, 0
		for i := range n {
			if ok[i] || i > 0 && i+1 < n && ok[i-1] && ok[i+1] {
				if length == 0 {
					start = i
				}
				length++
				if length > best.n {
					best = run{n: length, a: a0 + start, b: b0 + start}
				}
			} else {
				length = 0
			}
		}
	}
	return best
}

// segment is a span of a file in seconds.
type segment struct{ start, end float64 }

func (s segment) length() float64 { return s.end - s.start }

// sides are the segments a run marks in a and in b, offset by where each fingerprint
// starts in its file.
func (r run) sides(offsetA, offsetB float64) (segment, segment) {
	length := float64(r.n) * media.FingerprintItem
	a := offsetA + float64(r.a)*media.FingerprintItem
	b := offsetB + float64(r.b)*media.FingerprintItem
	return segment{a, a + length}, segment{b, b + length}
}

// agrees reports whether two finds place a segment alike: their starts or their ends
// within agreeWithin. One end is enough, because a pair loses the other to a dropout
// or to dialogue over the theme in one of its episodes.
func (s segment) agrees(o segment) bool {
	return math.Abs(o.start-s.start) <= agreeWithin || math.Abs(o.end-s.end) <= agreeWithin
}

// consensus is the segment of one file that at least two of its pairs found in the
// same place: the span they all share, at least minSegment long. Shorter finds are not
// candidates. With several groups the largest wins, then the longest.
func consensus(found []segment) (segment, bool) {
	var best segment
	bestN := 0
	for i, s := range found {
		if s.length() < minSegment {
			continue
		}
		shared, n := s, 1
		for j, o := range found {
			if j != i && o.length() >= minSegment && s.agrees(o) {
				shared = segment{max(shared.start, o.start), min(shared.end, o.end)}
				n++
			}
		}
		if n < 2 || shared.length() < minSegment {
			continue
		}
		if n > bestN || n == bestN && shared.length() > best.length() {
			best, bestN = shared, n
		}
	}
	return best, bestN > 0
}

// partners are the indexes of the files a file at i is matched with: the nearest ones
// in episode order that belong to another episode.
func partners(episodes []string, i int) []int {
	var out []int
	for d := 1; len(out) < pairsPerFile && (i-d >= 0 || i+d < len(episodes)); d++ {
		for _, j := range []int{i + d, i - d} {
			if j >= 0 && j < len(episodes) && episodes[j] != episodes[i] && len(out) < pairsPerFile {
				out = append(out, j)
			}
		}
	}
	return out
}

// sharedSegments finds, for every file of a season, the segment it shares with its
// partners: fps[i] is file i's fingerprint starting offsets[i] seconds into it,
// episodes[i] its episode. Pairs are matched once each.
func sharedSegments(fps [][]uint32, offsets []float64, episodes []string) []*segment {
	type pair struct{ i, j int }
	runs := map[pair]run{}
	out := make([]*segment, len(fps))
	for i := range fps {
		var found []segment
		for _, j := range partners(episodes, i) {
			p := pair{min(i, j), max(i, j)}
			r, ok := runs[p]
			if !ok {
				r = matchRun(fps[p.i], fps[p.j])
				runs[p] = r
			}
			a, b := r.sides(offsets[p.i], offsets[p.j])
			if i == p.i {
				found = append(found, a)
			} else {
				found = append(found, b)
			}
		}
		if s, ok := consensus(found); ok {
			out[i] = &s
		}
	}
	return out
}
