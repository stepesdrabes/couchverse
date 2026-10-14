package couchtv

import (
	"cmp"
	"slices"
	"strings"
	"unicode"
)

// Choosing the scenes of a trailer (D43) from the candidates of a title: the best, spread out
// over the title, saying different things, in story order, with a calm one to open on and a
// punchline to end on when the candidates have them.

// lineSpan is one line of a scene: when it is spoken (the file's time) and what is said, when
// that is known.
type lineSpan struct {
	From, To float64
	Text     string
}

// sceneCandidate is a stretch of a title's file that could be a scene, placed and rated.
type sceneCandidate struct {
	// File is the index of the title's file the picture is cut from.
	File       int
	Start, End float64
	Lines      []lineSpan
	// Score is 0 to 1; Opener and Button say how well the scene opens a trailer or ends it on
	// a punchline.
	Score, Opener, Button float64
	// Source is how its lines were found: subtitles or speech.
	Source string
	// words are what the lines say, for telling two scenes about the same thing apart.
	words map[string]bool
}

func (c sceneCandidate) length() float64 { return c.End - c.Start }

// What a trailer holds.
const (
	scenesMin, scenesMax = 6, 9
	// a trailer's scenes last this long together, at most
	trailerSeconds    = 60.0
	trailerSecondsMin = 40.0
	// a scene scoring less than this is not worth airing; below keepScore it only stays to make
	// up scenesKept scenes
	scoreFloor = 0.25
	keepScore  = 0.45
	scenesKept = 5
	// the opener and the button of a trailer must score at least this to be chosen for it
	structureFloor = 0.4
	// scenes of one file are at least this far apart: never two from the same minute or two
	sceneSpacing = 120.0
	// at most this many from one file of a series
	perEpisodeMax = 3
	// two scenes that say this much of the same are one thing said twice
	sameWords = 0.34
)

// contentWords are the words of a text that tell it from another: no stop words, no short ones.
func contentWords(text string) map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return !unicode.IsLetter(r) && r != '\'' }) {
		if len(w) >= 4 && !trailerStopWords[w] {
			out[w] = true
		}
	}
	return out
}

var trailerStopWords = wordSet(`that this with have what your from they them then there here were been will would could should
	about just like know want going gonna dont didnt cant thats its youre were well yeah okay right`)

// similar reports whether two scenes say much the same.
func similar(a, b sceneCandidate) bool {
	if len(a.words) == 0 || len(b.words) == 0 {
		return false
	}
	both := 0
	for w := range a.words {
		if b.words[w] {
			both++
		}
	}
	return float64(both)/float64(min(len(a.words), len(b.words))) >= sameWords && both >= 2
}

// chooseScenes picks the scenes of a trailer from candidates, in the order the title tells them
// (file, then time). files says how many files the title has; a series is cut from several.
func chooseScenes(cands []sceneCandidate, files int) []sceneCandidate {
	var usable []sceneCandidate
	for _, c := range cands {
		if c.Score >= scoreFloor {
			usable = append(usable, c)
		}
	}
	slices.SortStableFunc(usable, func(a, b sceneCandidate) int { return cmp.Compare(b.Score, a.Score) })

	// the best first, while they are far enough apart and say different things
	fits := func(chosen []sceneCandidate, c sceneCandidate) bool {
		perFile, total := 0, 0.0
		for _, o := range chosen {
			if o.File == c.File {
				perFile++
				if c.Start < o.End+sceneSpacing && o.Start < c.End+sceneSpacing {
					return false
				}
			}
			if similar(o, c) {
				return false
			}
			total += o.length()
		}
		return (files == 1 || perFile < perEpisodeMax) && total+c.length() <= trailerSeconds && len(chosen) < scenesMax
	}
	var chosen []sceneCandidate
	for _, c := range usable {
		if fits(chosen, c) {
			chosen = append(chosen, c)
		}
	}
	chosen = slices.Clone(chosen)
	inOrder := func(s []sceneCandidate) {
		slices.SortFunc(s, func(a, b sceneCandidate) int {
			return cmp.Or(cmp.Compare(a.File, b.File), cmp.Compare(a.Start, b.Start))
		})
	}
	inOrder(chosen)

	// the structure: swap the first for a calmer opener and the last for a punchline when
	// one is better, by more than it costs
	objective := func(s []sceneCandidate) float64 {
		if len(s) == 0 {
			return 0
		}
		total := 0.0
		for _, c := range s {
			total += c.Score
		}
		// a calm opener and a punchline count for what they are only when they are good scenes
		bonus := 0.0
		if s[0].Score >= structureFloor {
			bonus += 0.2 * s[0].Opener
		}
		if last := s[len(s)-1]; last.Score >= structureFloor {
			bonus += 0.2 * last.Button
		}
		return total + bonus
	}
	for range 40 {
		improved := false
		for k := range chosen {
			for _, c := range usable {
				if slices.ContainsFunc(chosen, func(o sceneCandidate) bool { return o.File == c.File && o.Start == c.Start }) {
					continue
				}
				without := slices.Delete(slices.Clone(chosen), k, k+1)
				if !fits(without, c) {
					continue
				}
				try := append(without, c)
				inOrder(try)
				if objective(try) > objective(chosen)+1e-9 {
					chosen, improved = try, true
					break
				}
			}
			if improved {
				break
			}
		}
		if !improved {
			break
		}
	}
	// a short trailer of strong scenes beats a long one with filler: weak scenes go, down to
	// scenesKept
	for len(chosen) > scenesKept {
		weakest := 0
		for k, c := range chosen {
			if c.Score < chosen[weakest].Score {
				weakest = k
			}
		}
		if chosen[weakest].Score >= keepScore {
			break
		}
		chosen = slices.Delete(chosen, weakest, weakest+1)
	}
	return chosen
}
