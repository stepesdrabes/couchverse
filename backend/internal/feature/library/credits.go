package library

import (
	"cmp"
	"math"
	"slices"

	"couchverse/internal/media"
)

// Credits detection for movies (docs/couch-tv-plan.md 3.1): the end credits look like
// text on black or a vertical crawl, and sound like a long stretch of steady music. Each
// detector misses some films on its own (designed credits, score under a final scene),
// so they are combined and the later answer wins: starting the squeeze late only shows
// more of the credits, starting it early would cut the ending.

// How the tail of a movie is sampled.
const (
	creditsFPS              = 2
	creditsW, creditsH      = 480, 270
	creditsStep             = 1.0 / creditsFPS
	maxScroll               = 40 // rows per sample, about 320 px/s at 1080p
	scrollColumnStep        = 3
	brightLuma, brightCount = 150, 6
)

// What credits look like on a sampled frame (the S3 thresholds, signalstats' scale).
const (
	cardMaxLuma       = 45
	cardMaxSaturation = 6
)

// A credits stretch lasts at least creditsSpan seconds with creditsShare of its frames
// looking like credits, and creditsRest of everything after its start does too (a
// post-credits scene is allowed to follow).
const (
	creditsSpan  = 60.0
	creditsShare = 0.85
	creditsRest  = 0.6
)

// Music under the credits: a steady stretch of at least musicMin seconds. On its own
// it is trusted only when it is long and runs to the end of the film, as credits music
// does.
const (
	musicMin      = 30.0
	musicAlone    = 90.0
	musicToTheEnd = 45.0
)

// look is what the detector reads from one sampled frame.
type look struct {
	// card: dark and desaturated with bright detail, i.e. text on black
	card bool
	// scroll: rows the picture moved up since the previous sample, 0 for none
	scroll int
}

// lookAt reads a frame (with chroma) against the one sampled before it (nil for the first).
func lookAt(prev *media.Frame, f media.Frame) look {
	l := look{card: isCard(f)}
	if prev != nil {
		l.scroll = scrollRows(*prev, f)
	}
	return l
}

func isCard(f media.Frame) bool {
	sum, bright := 0, 0
	for _, y := range f.Y {
		sum += int(y)
		if y > brightLuma {
			bright++
		}
	}
	if float64(sum)/float64(len(f.Y)) >= cardMaxLuma || bright < brightCount {
		return false
	}
	sat := 0.0
	for i := range f.U {
		u, v := float64(f.U[i])-128, float64(f.V[i])-128
		sat += math.Sqrt(u*u + v*v)
	}
	return sat/float64(len(f.U)) < cardMaxSaturation
}

// scrollRows finds how far the picture moved up between two samples: the shift whose
// difference beats the unmoved picture's clearly. A still or flat picture has none.
func scrollRows(prev, cur media.Frame) int {
	w, h := cur.Width, cur.Height
	rows := h - maxScroll
	diff := func(dy int) float64 {
		total := 0
		for y := range rows {
			a, b := cur.Y[y*w:y*w+w], prev.Y[(y+dy)*w:(y+dy)*w+w]
			for x := 0; x < w; x += scrollColumnStep {
				d := int(a[x]) - int(b[x])
				if d < 0 {
					d = -d
				}
				total += d
			}
		}
		return float64(total) / float64(rows*((w+scrollColumnStep-1)/scrollColumnStep))
	}
	still := diff(0)
	if still < 2 {
		return 0
	}
	best, bestAt := still, 0
	for dy := 1; dy <= maxScroll; dy++ {
		if d := diff(dy); d < best {
			best, bestAt = d, dy
		}
	}
	if bestAt == 0 || best >= still/2 || best >= 6 {
		return 0
	}
	return bestAt
}

// creditsLike says which samples look like credits: cards, and crawls that keep their
// speed from one sample to the next (a camera tilt rarely does).
func creditsLike(looks []look) []bool {
	steady := func(a, b int) bool { return a > 0 && b > 0 && a-b <= 1 && b-a <= 1 }
	out := make([]bool, len(looks))
	for i, l := range looks {
		crawl := i > 0 && steady(l.scroll, looks[i-1].scroll) || i+1 < len(looks) && steady(l.scroll, looks[i+1].scroll)
		out[i] = l.card || crawl
	}
	return out
}

// creditsStart is the first sample that starts a credits stretch, if any.
func creditsStart(like []bool) (int, bool) {
	sums := make([]int, len(like)+1)
	for i, v := range like {
		sums[i+1] = sums[i]
		if v {
			sums[i+1]++
		}
	}
	span := int(creditsSpan / creditsStep)
	for i := 0; i+span <= len(like); i++ {
		if like[i] && float64(sums[i+span]-sums[i]) >= creditsShare*float64(span) &&
			float64(sums[len(like)]-sums[i]) >= creditsRest*float64(len(like)-i) {
			return i, true
		}
	}
	return 0, false
}

// creditsMusic is the longest steady stretch of at least musicMin seconds.
func creditsMusic(l *media.Loudness) (media.Stretch, bool) {
	stretches := slices.DeleteFunc(l.Steady(), func(s media.Stretch) bool { return s.Length() < musicMin })
	if len(stretches) == 0 {
		return media.Stretch{}, false
	}
	return slices.MaxFunc(stretches, func(a, b media.Stretch) int { return cmp.Compare(a.Length(), b.Length()) }), true
}

// creditsAt decides where the credits start, in seconds into the analyzed tail (span
// seconds long, sampled into like). With both detectors the later answer wins unless
// the picture already shows credits all the way from the earlier one (the music of a
// crawl often settles later); the music alone counts only when it is long and runs to
// the end.
func creditsAt(like []bool, music media.Stretch, hasMusic bool, span float64) (float64, bool) {
	start, hasCrawl := creditsStart(like)
	crawl := float64(start) * creditsStep
	switch {
	case hasCrawl && hasMusic && music.Start > crawl:
		if share(like, crawl, music.Start) >= creditsShare {
			return crawl, true
		}
		return music.Start, true
	case hasCrawl:
		return crawl, true
	case hasMusic && music.Length() >= musicAlone && span-music.End <= musicToTheEnd:
		return music.Start, true
	}
	return 0, false
}

// share is the fraction of samples in [from, to) seconds that look like credits.
func share(like []bool, from, to float64) float64 {
	i, j := int(from/creditsStep), min(len(like), int(to/creditsStep))
	if j <= i {
		return 1
	}
	n := 0
	for _, v := range like[i:j] {
		if v {
			n++
		}
	}
	return float64(n) / float64(j-i)
}
