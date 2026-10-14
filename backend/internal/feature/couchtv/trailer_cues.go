package couchtv

import (
	"html"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"couchverse/internal/hls"
)

// A trailer's scenes (D43) are cut where lines begin and end, so the picker first reads what
// is said: the subtitle cues of the dialogue track's language, cleaned of everything that is
// not speech (sound descriptions, lyrics, speaker labels, markup), and the runs of cues that
// make a complete exchange a trailer could play.

// dialogueCue is a subtitle cue reduced to what is said.
type dialogueCue struct {
	Start, End float64
	// Text is the cue's lines joined by a space.
	Text  string
	Words int
	// Turns counts the speaker changes the cue marks itself: a dash line for each speaker
	// beyond the first.
	Turns int
}

// cueTrack is a subtitle file read for the picker.
type cueTrack struct {
	// Cues are the cues that carry speech, in order.
	Cues []dialogueCue
	// Songs are the stretches of lyrics; a scene never plays one (it would be a music video's
	// vocals, and the trailer's music cannot dip under it).
	Songs []timeSpan
	// Count is every cue of the file, speech or not: the fullest track of a language is the
	// one with the most.
	Count int
}

// timeSpan is a stretch of a title's time in seconds.
type timeSpan struct{ From, To float64 }

func (s timeSpan) overlaps(from, to float64) bool { return s.From < to && from < s.To }

var (
	cueTagRe = regexp.MustCompile(`<[^>]*>|\{\\[^}]*\}`)
	// sound descriptions and the like: [gunshots], (SIGHS), {music}
	cueSoundRe = regexp.MustCompile(`\[[^\]]*\]|\([^)]*\)|\{[^}]*\}`)
	// a speaker label in capitals, as SDH files write it: "TONY: ", "MAN #2 (O.S.): "
	cueLabelRe = regexp.MustCompile(`^[A-Z][A-Z0-9 .'#&/-]{0,23}(?: ?\([A-Z. ]{1,6}\))?:\s+`)
	cueDashRe  = regexp.MustCompile(`^[-\x{2013}\x{2014}]+\s*`)
	cueSpaceRe = regexp.MustCompile(`\s+`)
)

const lyricMarks = "♪♫♬"

// readCues reads a parsed WebVTT file: speech cues cleaned, lyrics set aside, on-screen
// captions ("NEW YORK CITY") dropped.
func readCues(v *hls.WebVTT) cueTrack {
	type raw struct {
		cue    hls.Cue
		lines  []string
		turns  int
		song   bool
		capped bool // every line is in capitals
	}
	var cues []raw
	mixed := 0
	for _, c := range v.Cues {
		r := raw{cue: c, capped: true}
		for _, line := range strings.Split(c.Payload, "\n") {
			line = html.UnescapeString(cueTagRe.ReplaceAllString(line, ""))
			if strings.ContainsAny(line, lyricMarks) {
				r.song = true
				continue
			}
			line = strings.TrimSpace(cueSoundRe.ReplaceAllString(line, ""))
			dash := cueDashRe.MatchString(line)
			line = cueDashRe.ReplaceAllString(line, "")
			line = strings.TrimSpace(cueLabelRe.ReplaceAllString(line, ""))
			line = strings.TrimSpace(cueSpaceRe.ReplaceAllString(line, " "))
			if line == "" || !strings.ContainsFunc(line, unicode.IsLetter) {
				continue
			}
			if dash {
				r.turns++
			}
			if strings.ToUpper(line) != line {
				r.capped = false
			}
			r.lines = append(r.lines, line)
		}
		if len(r.lines) > 0 && !r.capped {
			mixed++
		}
		cues = append(cues, r)
	}
	track := cueTrack{Count: len(v.Cues)}
	// a file in capitals throughout is just written that way; in a mixed file a line in
	// capitals without a stop is a caption
	captions := mixed*2 > len(cues)
	for _, r := range cues {
		if r.song {
			track.Songs = append(track.Songs, timeSpan{r.cue.Start, r.cue.End})
			continue
		}
		if len(r.lines) == 0 {
			continue
		}
		text := strings.Join(r.lines, " ")
		if captions && r.capped && !strings.ContainsAny(text, ".?!") {
			continue
		}
		dc := dialogueCue{Start: r.cue.Start, End: r.cue.End, Text: text, Words: len(strings.Fields(text))}
		if r.turns > 1 {
			dc.Turns = r.turns - 1
		}
		track.Cues = append(track.Cues, dc)
	}
	return track
}

// shifted is the track with every cue offset seconds later.
func (t cueTrack) shifted(offset float64) cueTrack {
	out := cueTrack{Count: t.Count, Cues: make([]dialogueCue, len(t.Cues)), Songs: make([]timeSpan, len(t.Songs))}
	for i, c := range t.Cues {
		c.Start, c.End = c.Start+offset, c.End+offset
		out.Cues[i] = c
	}
	for i, s := range t.Songs {
		out.Songs[i] = timeSpan{s.From + offset, s.To + offset}
	}
	return out
}

// Cues closer than this are one exchange: the next line answers or goes on.
const exchangeGap = 1.0

// speechMax is the most speech one scene holds; with its lead and tail it stays within the
// 8 s a scene may last.
const speechMax = 6.4

// textRun is consecutive cues of one exchange that make a whole thing to say.
type textRun struct {
	Cues []dialogueCue
	// Complete is how whole it ends: 1 on a full stop, question or exclamation mark, less on
	// an ellipsis.
	Complete float64
	// Before and After are the seconds of quiet (no cue) to the cue before and after it, at
	// most quietMax: where a scene can start and end without cutting a line.
	Before, After float64
}

// quietMax caps the quiet around a run; more than this is the same as this.
const quietMax = 5.0

func (r textRun) start() float64 { return r.Cues[0].Start }
func (r textRun) end() float64   { return r.Cues[len(r.Cues)-1].End }

func (r textRun) text() string {
	parts := make([]string, len(r.Cues))
	for i, c := range r.Cues {
		parts[i] = c.Text
	}
	return strings.Join(parts, " ")
}

func (r textRun) words() int {
	n := 0
	for _, c := range r.Cues {
		n += c.Words
	}
	return n
}

// beginsSentence reports whether a cue opens a thought rather than carrying on one: it starts
// with a capital, a digit, a quote or a dash, not in lower case.
func beginsSentence(c dialogueCue) bool {
	r, _ := utf8.DecodeRuneInString(strings.TrimLeft(c.Text, `"'“‘¿¡(`))
	return !unicode.IsLower(r)
}

// endCompleteness is how whole the thought is that a cue ends on: a stop, question or
// exclamation mark (a closing quote after it is fine) is whole; an ellipsis trails off; a
// comma, a dash or nothing is mid-sentence.
func endCompleteness(c dialogueCue) float64 {
	t := strings.TrimRight(c.Text, `"'”’) `)
	switch {
	case strings.HasSuffix(t, "...") || strings.HasSuffix(t, "…"):
		return 0.5
	case strings.HasSuffix(t, ".") || strings.HasSuffix(t, "?") || strings.HasSuffix(t, "!"):
		return 1
	}
	return 0
}

// textRuns lists the runs of an exchange that a scene could play: consecutive cues less than
// exchangeGap apart, starting on the start of a thought and ending on the end of one, with
// 1.2 to speechMax seconds of speech and at least three words. Songs split exchanges.
func textRuns(track cueTrack, from, to float64) []textRun {
	var out []textRun
	cues := track.Cues
	for i := range cues {
		if cues[i].Start < from || cues[i].End > to {
			continue
		}
		// a cue that carries on the one before it (no gap, a lower-case start) opens nothing
		if i > 0 && cues[i].Start-cues[i-1].End < exchangeGap && !beginsSentence(cues[i]) {
			continue
		}
		for j := i; j < len(cues); j++ {
			if j > i && cues[j].Start-cues[j-1].End >= exchangeGap {
				break
			}
			if cues[j].End > to || cues[j].End-cues[i].Start > speechMax {
				break
			}
			if overlapsSong(track.Songs, cues[i].Start, cues[j].End) {
				break
			}
			r := textRun{Cues: cues[i : j+1], Complete: endCompleteness(cues[j]), Before: quietMax, After: quietMax}
			if i > 0 {
				r.Before = min(quietMax, cues[i].Start-cues[i-1].End)
			}
			if j+1 < len(cues) {
				r.After = min(quietMax, cues[j+1].Start-cues[j].End)
			}
			if r.Complete == 0 || r.end()-r.start() < 1.2 || r.words() < 3 {
				continue
			}
			out = append(out, r)
		}
	}
	return out
}

// chunkMax is the longest stretch of dialogue decoded as one: a conversation that runs on is cut
// where its pauses are longest.
const chunkMax = 40.0

// exchangeChunks splits the cues within [from, to] into exchanges (cues less than exchangeGap
// apart) of at most chunkMax seconds, each as the stretch from its first cue's start to its
// last cue's end.
func exchangeChunks(cues []dialogueCue, from, to float64) []timeSpan {
	var out []timeSpan
	var group []dialogueCue
	flush := func() {
		for len(group) > 0 {
			n := len(group)
			if group[n-1].End-group[0].Start > chunkMax {
				// cut at the longest pause that leaves both parts something
				n, best := 0, -1.0
				for k := 1; k < len(group); k++ {
					if gap := group[k].Start - group[k-1].End; gap > best && group[k-1].End-group[0].Start <= chunkMax {
						n, best = k, gap
					}
				}
				if n == 0 {
					n = 1
				}
				out = append(out, timeSpan{group[0].Start, group[n-1].End})
				group = group[n:]
				continue
			}
			out = append(out, timeSpan{group[0].Start, group[n-1].End})
			return
		}
	}
	for _, c := range cues {
		if c.Start < from || c.End > to {
			continue
		}
		if len(group) > 0 && c.Start-group[len(group)-1].End >= exchangeGap {
			flush()
			group = nil
		}
		group = append(group, c)
	}
	flush()
	return out
}

func overlapsSong(songs []timeSpan, from, to float64) bool {
	for _, s := range songs {
		if s.overlaps(from, to) {
			return true
		}
	}
	return false
}
