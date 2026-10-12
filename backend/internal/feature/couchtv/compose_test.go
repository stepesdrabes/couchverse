package couchtv

import (
	"fmt"
	"slices"
	"testing"
	"time"
)

func specTitle(name string) SpecTitle {
	return SpecTitle{Name: name, Genres: []string{"Drama"}, Accents: Accents{Backdrop: "#3355aa"},
		Backdrop: "/media/v1/artwork/" + name, Poster: "/media/v1/artwork/" + name + "-poster"}
}

func breakProgramAt(name, fill string, start time.Time) *BreakProgram {
	return &BreakProgram{TitleID: name, Title: specTitle(name), Start: start, Fill: fill, Sub: "Movie · Drama · 120 min",
		Frames: []string{"/media/v1/frames/f/0_288/0.jpg"}, Bed: &Bed{URL: "/media/v1/audio/f/0_11520.flac", In: 0, Out: 11.52}}
}

// breakAt is a break of the given length ending at end, with a program on either side and a
// few titles to promote.
func breakAt(end time.Time, length time.Duration, nextFill string) BreakInput {
	start := end.Add(-length)
	stills := []Shot{{Kind: "still", Image: "backdrop", From: &Crop{0, 0, 1, 1}}}
	cands := []Candidate{
		{TitleID: "soon", Title: specTitle("soon"), Airs: start.Add(2 * time.Hour), Fill: FillMovie, Mood: "cinematic", Shots: stills, BodyFrames: 415},
		{TitleID: "later", Title: specTitle("later"), Airs: start.Add(50 * time.Hour), Fill: FillReruns, Mood: "bright", Shots: stills, BodyFrames: 330},
		{TitleID: "strip", Title: specTitle("strip"), Airs: start.Add(20 * time.Hour), Fill: FillStrip, StripMood: "bright", StripCount: 2},
		{TitleID: "unrendered", Title: specTitle("unrendered"), Airs: start.Add(5 * time.Hour), Fill: FillStrip, Mood: "grit", Shots: stills},
		{TitleID: "next", Title: specTitle("next"), Airs: end, Fill: nextFill, Mood: "grit", Shots: stills, BodyFrames: 400},
	}
	for i := range cands {
		cands[i].Airing = airingPhrase(cands[i].Airs, start, prague, cands[i].Fill)
	}
	cands[2].Strip = &SpecAiring{Eyebrow: "Every night", Time: "18:00", Days: []int{0, 1, 2, 3, 4, 5, 6}}
	later := []BreakProgram{*breakProgramAt("after", FillStrip, end.Add(30*time.Minute)), *breakProgramAt("late", FillReruns, end.Add(time.Hour))}
	return BreakInput{Start: start, Frames: int(length / (time.Second / FPS)), Loc: prague,
		Ending: breakProgramAt("ending", FillReruns, start.Add(-time.Hour)), Next: breakProgramAt("next", nextFill, end), Later: later,
		Candidates: cands, Recent: map[string]bool{}, Loudness: -20}
}

func kinds(c Composition) []string {
	out := make([]string, len(c.Parts))
	for i, p := range c.Parts {
		out[i] = p.Kind
	}
	return out
}

// checkComposition asserts what every break keeps: parts back to back summing to the gap,
// every spec valid against the broadcast schema, flexible templates within their bounds,
// and one sound item per clip that starts its own sound.
func checkComposition(t *testing.T, in BreakInput, c Composition) {
	t.Helper()
	at := 0
	for i, p := range c.Parts {
		if p.At != at || p.Frames <= 0 {
			t.Fatalf("part %d (%s) at %d for %d frames, want it at %d", i, p.Kind, p.At, p.Frames, at)
		}
		at += p.Frames
		if err := validateSpec(p.Spec); err != nil {
			t.Fatalf("part %d (%s): %v", i, p.Kind, err)
		}
		bounds := map[string]span{ClipSting: stingSpan, ClipUpNext: upNextSpan, ClipLineup: lineupSpan}
		if s, ok := bounds[p.Kind]; ok && (p.Frames < s.min || p.Frames > s.max) {
			t.Errorf("%s of %d frames", p.Kind, p.Frames)
		}
		if _, ok := bounds[p.Kind]; ok && framesOf(p.Spec.Duration) != p.Frames {
			t.Errorf("%s spec says %gs for %d frames", p.Kind, p.Spec.Duration, p.Frames)
		}
	}
	if at != in.Frames || c.Frames != in.Frames {
		t.Fatalf("the parts sum to %d frames (%d), the gap is %d", at, c.Frames, in.Frames)
	}
	if err := validateSpec(c.Sound); err != nil {
		t.Fatalf("break sound: %v", err)
	}
	if c.Sound.Loudness < quietestLUFS || c.Sound.Loudness > loudestLUFS {
		t.Errorf("loudness %g", c.Sound.Loudness)
	}
}

func TestComposeFillsEveryGapExactly(t *testing.T) {
	end := time.Date(2026, 10, 14, 18, 25, 0, 0, prague)
	// every length from the shortest break up to half an hour, frame by frame near the edges
	var lengths []time.Duration
	for f := 50; f <= 2000; f++ {
		lengths = append(lengths, time.Duration(f)*time.Second/FPS)
	}
	for s := 80; s <= 30*60; s += 7 {
		lengths = append(lengths, time.Duration(s)*time.Second+40*time.Millisecond*time.Duration(s%25))
	}
	for _, length := range lengths {
		in := breakAt(end, length, FillStrip)
		c, err := Compose(in)
		if err != nil {
			t.Fatalf("%v: %v", length, err)
		}
		checkComposition(t, in, c)
	}
	in := breakAt(end, 40*time.Millisecond*49, FillStrip)
	if _, err := Compose(in); err != errTooShort {
		t.Errorf("49 frames: %v", err)
	}
}

func TestComposeMix(t *testing.T) {
	tests := []struct {
		name   string
		end    time.Time
		length time.Duration
		fill   string
		want   []string
	}{
		{"the shortest break", time.Date(2026, 10, 14, 18, 25, 0, 0, prague), 22 * time.Second, FillStrip,
			[]string{ClipSqueeze, ClipIdentShort, ClipUpNext}},
		{"too short for an ident", time.Date(2026, 10, 14, 18, 25, 0, 0, prague), 20 * time.Second, FillStrip,
			[]string{ClipSqueeze, ClipUpNext}},
		{"a sting", time.Date(2026, 10, 14, 18, 25, 0, 0, prague), 3 * time.Second, FillStrip,
			[]string{ClipSting}},
		{"up next absorbs the rest", time.Date(2026, 10, 14, 18, 25, 0, 0, prague), 44 * time.Second, FillStrip,
			[]string{ClipSqueeze, ClipIdentShort, ClipPromoBody, ClipPromoEnd, ClipUpNext}},
		{"a sting where up next cannot stretch", time.Date(2026, 10, 14, 18, 25, 0, 0, prague), 50 * time.Second, FillStrip,
			[]string{ClipSqueeze, ClipIdentShort, ClipPromoBody, ClipPromoEnd, ClipSting, ClipUpNext}},
		{"three promos", time.Date(2026, 10, 14, 18, 25, 0, 0, prague), 75 * time.Second, FillStrip,
			[]string{ClipSqueeze, ClipIdentShort, ClipPromoBody, ClipPromoEnd, ClipShortPromo, ClipStripPromo, ClipSting, ClipUpNext}},
		{"the full ident before the movie", time.Date(2026, 10, 14, 20, 15, 0, 0, prague), 60 * time.Second, FillMovie,
			[]string{ClipSqueeze, ClipIdentFull, ClipPromoBody, ClipPromoEnd, ClipShortPromo, ClipUpNext}},
		{"no lineup without room for a promo", time.Date(2026, 10, 14, 18, 0, 0, 0, prague), 40 * time.Second, FillStrip,
			[]string{ClipSqueeze, ClipIdentShort, ClipShortPromo, ClipShortPromo, ClipUpNext}},
		{"the lineup on the hour", time.Date(2026, 10, 14, 18, 0, 0, 0, prague), 90 * time.Second, FillStrip,
			[]string{ClipSqueeze, ClipIdentShort, ClipPromoBody, ClipPromoEnd, ClipShortPromo, ClipStripPromo, ClipShortPromo, ClipLineup, ClipUpNext}},
	}
	for _, tt := range tests {
		in := breakAt(tt.end, tt.length, tt.fill)
		c, err := Compose(in)
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		checkComposition(t, in, c)
		if got := kinds(c); !slices.Equal(got, tt.want) {
			t.Errorf("%s: %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestComposePromos(t *testing.T) {
	end := time.Date(2026, 10, 14, 18, 25, 0, 0, prague)
	in := breakAt(end, 4*time.Minute, FillStrip)
	c, err := Compose(in)
	if err != nil {
		t.Fatal(err)
	}
	checkComposition(t, in, c)
	// the movie tonight, the strips by how soon they air (the unrendered one as a short
	// promo), the rerun; the next program is up next's, never a promo
	if want := []string{"soon", "unrendered", "strip", "later"}; !slices.Equal(c.Promos[:4], want) {
		t.Errorf("promos %v, want %v first", c.Promos, want)
	}
	if slices.Contains(c.Promos, "next") {
		t.Errorf("the next program is promoted: %v", c.Promos)
	}
	for _, p := range c.Parts {
		if p.TitleID == "unrendered" && p.Kind != ClipShortPromo || p.TitleID == "strip" && p.Kind != ClipStripPromo {
			t.Errorf("%s promoted with %s", p.TitleID, p.Kind)
		}
		if p.Kind == ClipPromoBody && (p.Spec.Airing != nil || p.Sound.Part != "full" || p.Sound.Airing == nil) {
			t.Errorf("a body carries an airing, or its sound is not the whole promo: %+v", p)
		}
	}

	// titles promoted in the breaks before go last
	in.Recent = map[string]bool{"soon": true}
	c, _ = Compose(in)
	if c.Promos[3] != "soon" {
		t.Errorf("recent title not last: %v", c.Promos)
	}

	// a premiere outranks a sooner movie
	in = breakAt(end, 4*time.Minute, FillStrip)
	in.Candidates[1].Fill = FillPremiere
	in.Candidates[1].Airs = in.Start.Add(26 * time.Hour)
	c, _ = Compose(in)
	if c.Promos[0] != "later" {
		t.Errorf("the premiere is not first: %v", c.Promos)
	}
	for _, p := range c.Parts {
		if p.TitleID == "later" && p.Kind == ClipPromoBody && !p.Spec.Premiere {
			t.Error("the premiere's body is not the premiere variant")
		}
	}

	// a long gap starts another round after a short ident
	in = breakAt(end, 12*time.Minute, FillStrip)
	c, _ = Compose(in)
	checkComposition(t, in, c)
	if n := slices.Index(c.Promos, "soon"); n < 0 || slices.Index(c.Promos[n+1:], "soon") < 0 {
		t.Errorf("one round only: %v", c.Promos)
	}
}

func TestComposeClocksAndLoudness(t *testing.T) {
	end := time.Date(2026, 10, 14, 18, 0, 0, 0, prague)
	in := breakAt(end, 90*time.Second, FillStrip)
	in.Loudness = -30
	c, err := Compose(in)
	if err != nil {
		t.Fatal(err)
	}
	if c.Sound.Loudness != quietestLUFS {
		t.Errorf("loudness %g, want it clamped to %g", c.Sound.Loudness, quietestLUFS)
	}
	squeeze, up := c.Parts[0], c.Parts[len(c.Parts)-1]
	if squeeze.Spec.Clock != "17:58" || up.Spec.Clock != "17:59" || up.Spec.Time != "18:00" {
		t.Errorf("squeeze clock %s, up next clock %s at %s", squeeze.Spec.Clock, up.Spec.Clock, up.Spec.Time)
	}
	for _, p := range c.Parts {
		if p.Kind == ClipLineup && (p.Spec.Items[0].State != "next" || p.Spec.Items[0].Badge != "Next · in 2 min") {
			t.Errorf("lineup %+v", p.Spec.Items[0])
		}
	}
	in.Loudness = 0
	if c, _ := Compose(in); c.Sound.Loudness != defaultLUFS {
		t.Errorf("unknown loudness mixes at %g", c.Sound.Loudness)
	}
}

func TestAiringPhrases(t *testing.T) {
	from := time.Date(2026, 10, 14, 12, 0, 0, 0, prague) // a Wednesday
	tests := []struct {
		at   time.Time
		fill string
		want SpecAiring
	}{
		{time.Date(2026, 10, 14, 15, 0, 0, 0, prague), FillReruns, SpecAiring{Eyebrow: "Today", Time: "15:00"}},
		{time.Date(2026, 10, 14, 20, 15, 0, 0, prague), FillMovie, SpecAiring{Eyebrow: "Tonight", Time: "20:15"}},
		{time.Date(2026, 10, 15, 18, 0, 0, 0, prague), FillStrip, SpecAiring{Eyebrow: "Tomorrow", Time: "18:00"}},
		{time.Date(2026, 10, 17, 20, 15, 0, 0, prague), FillPremiere, SpecAiring{Eyebrow: "Premiere", Day: "Saturday", Time: "20:15"}},
		{time.Date(2026, 10, 18, 20, 15, 0, 0, prague), FillSaga, SpecAiring{Eyebrow: "This week", Day: "Sunday", Time: "20:15"}},
	}
	for _, tt := range tests {
		if got := airingPhrase(tt.at, from, prague, tt.fill); fmt.Sprint(got) != fmt.Sprint(tt.want) {
			t.Errorf("%s: %+v, want %+v", tt.at, got, tt.want)
		}
	}
	if got := stripPhrase([]string{"mon", "tue", "wed", "thu", "fri"}, "18:00"); got.Eyebrow != "Every weekday" || !slices.Equal(got.Days, []int{0, 1, 2, 3, 4}) {
		t.Errorf("weekday strip: %+v", got)
	}
	if got := stripPhrase(dayNames, "18:00"); got.Eyebrow != "Every night" || len(got.Days) != 7 {
		t.Errorf("nightly strip: %+v", got)
	}
}

func TestComposeEvergreen(t *testing.T) {
	stills := []Shot{{Kind: "still", Image: "backdrop", From: &Crop{0, 0, 1, 1}}}
	c := ComposeEvergreen([]Candidate{
		{TitleID: "a", Title: specTitle("a"), Fill: FillPremiere, Mood: "grit", Shots: stills, BodyFrames: 330},
		{TitleID: "b", Title: specTitle("b"), Mood: "bright", Shots: stills, BodyFrames: 340},
	})
	want := []string{ClipIdentFull, ClipPromoBody, ClipEvergreenEnd, ClipPromoBody, ClipEvergreenEnd, ClipIdentShort}
	if !slices.Equal(kinds(c), want) {
		t.Errorf("%v, want %v", kinds(c), want)
	}
	if c.Frames != identFullFrames+330+340+2*endCardFrames+identShortFrames {
		t.Errorf("%d frames", c.Frames)
	}
	for _, p := range c.Parts {
		if err := validateSpec(p.Spec); err != nil {
			t.Errorf("%s: %v", p.Kind, err)
		}
		if p.Spec.Premiere || p.Spec.Airing != nil {
			t.Errorf("%s depends on an airing: %+v", p.Kind, p.Spec)
		}
	}
	if err := validateSpec(c.Sound); err != nil {
		t.Error(err)
	}
}
