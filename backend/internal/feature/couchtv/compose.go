package couchtv

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"slices"
	"time"
)

// Break composition (plan 6.5, D17) is a pure function from a break's length and what airs
// around it to the clips that fill it, in order and to the frame, and the break's sound as
// one break spec. A normal break is the credits squeeze, an ident (the full cut before a
// prime-time movie), as many promos as fit (a body with its end card, a short promo, or a
// strip promo), the lineup in a break that ends on the hour, and up next; up next, the
// lineup and stings (any whole number of frames from 2 to 6 s) absorb what is left.

// Template lengths in frames, as the broadcast package builds them.
const (
	identFullFrames     = 263 // 10.52 s
	identShortFrames    = 113 // 4.52 s
	squeezeCursorFrames = 265 // 10.6 s, collapsing into the short ident's cursor
	squeezeExpandFrames = 288 // 11.52 s, handing over to up next
	endCardFrames       = 100 // a promo's end card
	shortPromoFrames    = 225 // the short promo's default
	stripPromoFrames    = 400 // the strip promo's default
	standbyFrames       = 300 // one loop of the standby card
)

// span is a template whose length the composer chooses: from min to max, def when there
// is nothing to absorb.
type span struct{ min, def, max int }

var (
	upNextSpan = span{150, 200, 300}
	lineupSpan = span{250, 300, 400}
	stingSpan  = span{50, 75, 150}
)

// Loudness bounds a break is mixed at (6.5), and the level when the program's is unknown.
const (
	quietestLUFS  = -23.0
	loudestLUFS   = -16.0
	defaultLUFS   = -18.0
	lineupRowsMax = 5
)

// errTooShort means a gap no clip can fill (under a 2 s sting).
var errTooShort = errors.New("couchtv: the break is shorter than the shortest sting")

// BreakProgram is a program next to a break, as the break shows it.
type BreakProgram struct {
	TitleID string
	Title   SpecTitle
	Episode *SpecEpisode
	Start   time.Time
	Fill    string
	// Sub is its lineup line (`S01E02 · Name`, `Movie · Drama · 126 min`).
	Sub string
	// Frames are the program's frames from its credits for the squeeze panel; nil shows its
	// backdrop instead. Bed is its sound from the credits.
	Frames []string
	Bed    *Bed
	// Promo is its soundtrack excerpt, which up next plays under itself.
	Promo *Bed
}

// Candidate is a title a break may promote, at its next airing after the break.
type Candidate struct {
	TitleID string
	Title   SpecTitle
	Airs    time.Time
	Fill    string
	// Airing is the end card's and the short promo's phrase.
	Airing SpecAiring
	// Strip is set for a strip at a fixed time: the strip promo's phrase and its days.
	Strip      *SpecAiring
	StripCount int
	Mood       string // the promo's: cinematic, grit, bright
	StripMood  string // the strip promo's: bright, dark
	Shots      []Shot
	Bed        *Bed
	// BodyFrames is the length of the title's rendered promo body; 0 until it is rendered.
	BodyFrames int
}

// BreakInput is everything a break's composition depends on.
type BreakInput struct {
	Start  time.Time
	Frames int
	Loc    *time.Location
	Ending *BreakProgram
	Next   *BreakProgram
	// Later are the programs after Next, for the lineup.
	Later      []BreakProgram
	Candidates []Candidate
	// Recent are the titles promoted in the breaks just before this one.
	Recent   map[string]bool
	Loudness float64
}

// Part is one clip of a break.
type Part struct {
	Kind   string
	Spec   Spec
	At     int // frames into the break
	Frames int
	// Sound is what the break's sound plays from this part on; nil when the part before
	// rings on under it (a promo's end card continues its body's render).
	Sound *Spec
	// TitleID is the title a promo part promotes.
	TitleID string
}

// Composition is a composed break.
type Composition struct {
	Frames int
	Parts  []Part
	Sound  BreakSpec
	// Promos are the titles it promotes, in order.
	Promos []string
}

// piece is a part in the making: a fixed length, or one the fit sizes.
type piece struct {
	part Part
	flex *span
}

// Compose fills a break.
func Compose(in BreakInput) (Composition, error) {
	g := in.Frames
	var head, tail []piece
	var groups [][]piece
	// left counts the flexible parts at their defaults; slack is what they can give back
	left, slack := g, 0
	normal := g >= squeezeCursorFrames+identShortFrames+upNextSpan.min

	switch {
	case in.Ending != nil && in.Next != nil && !normal && g >= squeezeExpandFrames+upNextSpan.min:
		// too short for an ident: the next picture expands out of the squeeze into up next
		head = append(head, fixed(squeezePart(in, "expand")))
		tail = append(tail, upNextPiece(in, "continue"))
		left -= squeezeExpandFrames + upNextSpan.def
	case normal || (in.Ending == nil || in.Next == nil) && g >= identShortFrames+upNextSpan.min:
		if in.Ending != nil && normal {
			head = append(head, fixed(squeezePart(in, "cursor")))
			left -= squeezeCursorFrames
		}
		if in.Next != nil {
			tail = append(tail, upNextPiece(in, "cut"))
			left -= upNextSpan.def
			slack += upNextSpan.def - upNextSpan.min
		}
		ident := identPart(false)
		if in.Next != nil && primeTime(in.Next.Fill) && left+slack >= identFullFrames {
			ident = identPart(true)
		}
		head = append(head, fixed(ident))
		left -= ident.Frames
		// the lineup only where a promo still fits beside it
		if items := lineupItems(in); len(items) >= 2 && endsOnTheHour(in) && left >= lineupSpan.def+shortPromoFrames {
			tail = slices.Insert(tail, 0, piece{part: Part{Kind: ClipLineup, Spec: Spec{Template: "lineup", Items: items}}, flex: &lineupSpan})
			left -= lineupSpan.def
			slack += lineupSpan.def - lineupSpan.min
		}
		groups, left = promos(in, left, slack)
	case in.Next != nil && g >= upNextSpan.min:
		tail = append(tail, upNextPiece(in, "cut"))
		left -= upNextSpan.def
	}

	// everything flexible takes up what is left, with stings where it cannot stretch
	var flex []*piece
	for i := range tail {
		if tail[i].flex != nil {
			flex = append(flex, &tail[i])
		}
	}
	need := left
	for _, f := range flex {
		need += f.flex.def
	}
	spans := make([]span, len(flex))
	for i, f := range flex {
		spans[i] = *f.flex
	}
	sizes, stings, ok := fit(spans, need)
	if !ok {
		return Composition{}, errTooShort
	}
	for i, f := range flex {
		f.part.Frames = sizes[i]
		f.part.Spec.Duration = frameSeconds(sizes[i])
	}
	stingPieces := make([]piece, stings)
	for i := range stingPieces {
		n := sizes[len(flex)+i]
		stingPieces[i] = fixed(Part{Kind: ClipSting, Spec: Spec{Template: "sting", Duration: frameSeconds(n)}, Frames: n})
	}

	// stings close the gap before the lineup and up next, then between promos
	seq := slices.Clone(head)
	slots := make([][]piece, len(groups)+1)
	for i, s := range stingPieces {
		slot := len(slots) - 1 - i%len(slots)
		slots[slot] = append(slots[slot], s)
	}
	seq = append(seq, slots[0]...)
	for i, grp := range groups {
		seq = append(seq, grp...)
		seq = append(seq, slots[i+1]...)
	}
	seq = append(seq, tail...)
	return assemble(in, seq), nil
}

func fixed(p Part) piece { return piece{part: p} }

// assemble lays the parts out and writes the break's sound.
func assemble(in BreakInput, seq []piece) Composition {
	c := Composition{Sound: BreakSpec{Template: "break", Loudness: breakLoudness(in.Loudness)}}
	at := 0
	for _, p := range seq {
		part := p.part
		part.At = at
		at += part.Frames
		if in.Loc != nil && (part.Kind == ClipSqueeze || part.Kind == ClipUpNext) {
			// the corner bug's clock, as of the part's first frame
			part.Spec.Clock = localTime(in.Start.Add(time.Duration(part.At)*time.Second/FPS), in.Loc)
		}
		if part.Kind != ClipPromoEnd && part.Kind != ClipEvergreenEnd {
			if part.Sound == nil {
				s := part.Spec
				part.Sound = &s
			}
			c.Sound.Items = append(c.Sound.Items, BreakItem{At: frameSeconds(part.At), Spec: *part.Sound})
		}
		if part.TitleID != "" && (len(c.Promos) == 0 || c.Promos[len(c.Promos)-1] != part.TitleID) {
			c.Promos = append(c.Promos, part.TitleID)
		}
		c.Parts = append(c.Parts, part)
	}
	c.Frames = at
	return c
}

// fit sizes the flexible parts so they and some stings fill need frames: the fewest
// stings that make it fit, every part at its default before any grows further. It fails
// when need is below the parts' minimum.
func fit(spans []span, need int) (sizes []int, stings int, ok bool) {
	for ; ; stings++ {
		all := slices.Clone(spans)
		for range stings {
			all = append(all, stingSpan)
		}
		var lo, hi int
		for _, s := range all {
			lo += s.min
			hi += s.max
		}
		if need < lo {
			return nil, 0, false
		}
		if need > hi {
			continue
		}
		sizes = make([]int, len(all))
		extra := need - lo
		for i, s := range all {
			sizes[i] = s.min
		}
		for _, limit := range []func(span) int{func(s span) int { return s.def }, func(s span) int { return s.max }} {
			for i, s := range all {
				add := min(extra, limit(s)-sizes[i])
				if add > 0 {
					sizes[i] += add
					extra -= add
				}
			}
		}
		return sizes, stings, true
	}
}

// promos fills the space left (and the slack the flexible parts can give back) with
// promos, the best candidates first; a gap longer than one round of the candidates starts
// another after a short ident.
func promos(in BreakInput, left, slack int) ([][]piece, int) {
	ranked := rankCandidates(in)
	var groups [][]piece
	for round := 0; len(ranked) > 0; round++ {
		added := false
		for _, c := range ranked {
			grp, n := promoPieces(c, left+slack)
			if n == 0 {
				continue
			}
			if round > 0 && !added {
				// a new round opens with the short ident, if it still fits
				if left+slack < n+identShortFrames {
					continue
				}
				groups = append(groups, []piece{fixed(identPart(false))})
				left -= identShortFrames
			}
			groups = append(groups, grp)
			left -= n
			added = true
		}
		if !added {
			break
		}
	}
	// a long gap with nothing to promote is the full ident on repeat, not a run of stings
	for left >= identFullFrames+2*stingSpan.max {
		groups = append(groups, []piece{fixed(identPart(true))})
		left -= identFullFrames
	}
	return groups, left
}

// promoPieces is the biggest promo of a candidate that fits in left frames: the full promo
// (its rendered body and an end card) or the strip promo, else the short promo.
func promoPieces(c Candidate, left int) ([]piece, int) {
	title := c.Title
	switch {
	case c.Strip != nil && left >= stripPromoFrames:
		spec := Spec{Template: "stripPromo", Mood: c.StripMood, Title: &title, Airing: c.Strip,
			Duration: frameSeconds(stripPromoFrames), Bed: c.Bed}
		if c.StripCount > 1 {
			spec.Foot = &StripFoot{Kind: "count", Count: c.StripCount}
		}
		return []piece{fixed(Part{Kind: ClipStripPromo, Spec: spec, Frames: stripPromoFrames, TitleID: c.TitleID})}, stripPromoFrames
	case c.Strip == nil && c.BodyFrames > 0 && left >= c.BodyFrames+endCardFrames:
		body := promoBody(c)
		end := body
		end.Part = "end"
		airing := c.Airing
		end.Airing = &airing
		full := end
		full.Part = "full"
		return []piece{
			fixed(Part{Kind: ClipPromoBody, Spec: body, Frames: c.BodyFrames, Sound: &full, TitleID: c.TitleID}),
			fixed(Part{Kind: ClipPromoEnd, Spec: end, Frames: endCardFrames, TitleID: c.TitleID}),
		}, c.BodyFrames + endCardFrames
	case left >= shortPromoFrames:
		airing := c.Airing
		spec := Spec{Template: "shortPromo", Title: &title, Airing: &airing, Duration: frameSeconds(shortPromoFrames), Bed: c.Bed}
		return []piece{fixed(Part{Kind: ClipShortPromo, Spec: spec, Frames: shortPromoFrames, TitleID: c.TitleID})}, shortPromoFrames
	}
	return nil, 0
}

// promoBody is a candidate's promo body: timeless, so it renders once per title.
func promoBody(c Candidate) Spec {
	title := c.Title
	return Spec{Template: "promo", Part: "body", Mood: c.Mood, Premiere: c.Fill == FillPremiere, Title: &title,
		Shots: c.Shots, Bed: c.Bed}
}

// rankCandidates orders the promos (6.5): a fill's weight over one plus the days until it
// airs, so tonight's movie leads and a Saturday premiere (D24) rises through the week
// until it does; the next program is left out (up next announces it) and the titles of
// the last breaks go after all others.
func rankCandidates(in BreakInput) []Candidate {
	type scored struct {
		c      Candidate
		score  float64
		recent bool
	}
	var list []scored
	for _, c := range in.Candidates {
		if in.Next != nil && c.TitleID == in.Next.TitleID {
			continue
		}
		hours := max(0, c.Airs.Sub(in.Start).Hours())
		list = append(list, scored{c: c, score: fillWeight(c.Fill) / (1 + hours/24), recent: in.Recent[c.TitleID]})
	}
	slices.SortStableFunc(list, func(a, b scored) int {
		if a.recent != b.recent {
			if a.recent {
				return 1
			}
			return -1
		}
		if a.score != b.score {
			return cmp.Compare(b.score, a.score)
		}
		return cmp.Compare(a.c.TitleID, b.c.TitleID)
	})
	out := make([]Candidate, len(list))
	for i, s := range list {
		out[i] = s.c
	}
	return out
}

func fillWeight(fill string) float64 {
	switch fill {
	case FillPremiere:
		return 6
	case FillMovie, FillSaga:
		return 2
	case FillStrip, FillMarathon:
		return 1.5
	case FillMatinee:
		return 1
	}
	return 0.4
}

// primeTime reports whether a program is the evening's movie, which gets the full ident.
func primeTime(fill string) bool {
	return fill == FillMovie || fill == FillPremiere || fill == FillSaga
}

func endsOnTheHour(in BreakInput) bool {
	return in.Next != nil && in.Next.Start.In(in.Loc).Minute() == 0
}

func identPart(full bool) Part {
	if full {
		return Part{Kind: ClipIdentFull, Spec: Spec{Template: "ident", Cut: "full"}, Frames: identFullFrames}
	}
	return Part{Kind: ClipIdentShort, Spec: Spec{Template: "ident", Cut: "short"}, Frames: identShortFrames}
}

func squeezePart(in BreakInput, handover string) Part {
	frames := squeezeCursorFrames
	if handover == "expand" {
		frames = squeezeExpandFrames
	}
	e, n := in.Ending, in.Next
	program := &ProgramSource{Kind: "frames", Frames: e.Frames}
	if len(e.Frames) == 0 {
		// without credits markers the program aired in full: its backdrop, with no roll
		program = &ProgramSource{Kind: "standIn", Credits: &[][]string{}}
	}
	ending := e.Title
	return Part{Kind: ClipSqueeze, Frames: frames, Spec: Spec{
		Template: "squeeze", Ending: &ending, Program: program,
		Next:     &SqueezeNext{Title: n.Title, Episode: n.Episode, Time: localTime(n.Start, in.Loc)},
		Handover: handover, Bed: e.Bed,
	}}
}

func upNextPiece(in BreakInput, enter string) piece {
	n := in.Next
	title := n.Title
	spec := Spec{Template: "upNext", Title: &title, Episode: n.Episode, Time: localTime(n.Start, in.Loc), Bed: n.Promo}
	if enter == "continue" {
		spec.Enter = enter
	}
	return piece{part: Part{Kind: ClipUpNext, Spec: spec}, flex: &upNextSpan}
}

// lineupItems are the programs from the next one on, the next one counting down.
func lineupItems(in BreakInput) []LineupItem {
	if in.Next == nil {
		return nil
	}
	all := append([]BreakProgram{*in.Next}, in.Later...)
	var items []LineupItem
	for i, p := range all {
		if i == lineupRowsMax {
			break
		}
		item := LineupItem{Time: localTime(p.Start, in.Loc), Title: p.Title, Sub: p.Sub}
		if i == 0 {
			item.State = "next"
			minutes := int(math.Ceil(p.Start.Sub(in.Start).Minutes()))
			item.Badge = fmt.Sprintf("Next · in %d min", max(1, minutes))
		}
		items = append(items, item)
	}
	return items
}

// breakLoudness mixes a break at the loudness of the program around it, within bounds.
func breakLoudness(program float64) float64 {
	if program == 0 {
		return defaultLUFS
	}
	return math.Round(min(loudestLUFS, max(quietestLUFS, program))*10) / 10
}

// localTime is a 24-hour time in the channel's zone.
func localTime(t time.Time, loc *time.Location) string {
	return t.In(loc).Format("15:04")
}

// weekdays are the spec's day indexes from Monday.
func specWeekday(d time.Weekday) int { return (int(d) + 6) % 7 }

var weekdayNames = []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}

// airingPhrase says when a program airs, seen from a break (clients/broadcast airing.ts).
func airingPhrase(at, from time.Time, loc *time.Location, fill string) SpecAiring {
	a, f := at.In(loc), from.In(loc)
	day := weekdayNames[specWeekday(a.Weekday())]
	phrase := SpecAiring{Time: a.Format("15:04")}
	days := int(math.Round(civil(a).Sub(civil(f)).Hours() / 24))
	switch {
	case fill == FillPremiere:
		phrase.Eyebrow, phrase.Day = "Premiere", day
	case days == 0 && a.Hour() >= 17:
		phrase.Eyebrow = "Tonight"
	case days == 0:
		phrase.Eyebrow = "Today"
	case days == 1:
		phrase.Eyebrow = "Tomorrow"
	default:
		phrase.Eyebrow, phrase.Day = "This week", day
	}
	return phrase
}

// stripPhrase is a strip promo's phrase for a strip on the given days at hhmm.
func stripPhrase(days []string, hhmm string) SpecAiring {
	var idx []int
	for _, d := range days {
		if i := slices.Index(dayNames, d); i >= 0 {
			idx = append(idx, specWeekday(time.Weekday(i)))
		}
	}
	slices.Sort(idx)
	phrase := SpecAiring{Eyebrow: "Every week", Time: hhmm, Days: idx}
	switch {
	case len(idx) == 7 && hhmm >= "17:00":
		phrase.Eyebrow = "Every night"
	case len(idx) == 7:
		phrase.Eyebrow = "Every day"
	case slices.Equal(idx, []int{0, 1, 2, 3, 4}):
		phrase.Eyebrow = "Every weekday"
	}
	return phrase
}

func civil(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// ComposeEvergreen is the fallback break the live stream airs when a break is not ready:
// the full ident, promo bodies with the evergreen end card, the short ident. Every clip
// is airing-independent, so it is rendered once and kept.
func ComposeEvergreen(bodies []Candidate) Composition {
	seq := []piece{fixed(identPart(true))}
	for _, c := range bodies {
		c.Fill = "" // a premiere's body says so; the evergreen break airs any time
		body := promoBody(c)
		end := body
		end.Template, end.Part = "evergreenEnd", ""
		full := body
		full.Part = "full"
		seq = append(seq,
			fixed(Part{Kind: ClipPromoBody, Spec: body, Frames: c.BodyFrames, Sound: &full, TitleID: c.TitleID}),
			fixed(Part{Kind: ClipEvergreenEnd, Spec: end, Frames: endCardFrames, TitleID: c.TitleID}))
	}
	seq = append(seq, fixed(identPart(false)))
	return assemble(BreakInput{}, seq)
}
