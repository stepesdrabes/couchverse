package couchtv

import (
	"slices"
	"strings"
	"testing"
	"time"
)

// trailerScenes are n scenes of 1 s (the shortest) from stored scenes, as the composer builds them.
func trailerScenes(n int) []SpecScene {
	stream := 1
	var stored []CouchTvTrailerScene
	for i := range n {
		s := CouchTvTrailerScene{MediaFileID: "0f5c1a52-8e6d-4c1a-9a4e-3d2f4c7b9e11", AudioFileID: "0f5c1a52-8e6d-4c1a-9a4e-3d2f4c7b9e11",
			StartMs: 600_000 + i*300_000, DurationMs: 1000, Lines: []CouchTvTrailerLine{{FromMs: 100, ToMs: 400, Text: "Hello."}, {FromMs: 800, ToMs: 950}}}
		if i%2 == 0 {
			s.AudioStream = &stream
		}
		stored = append(stored, s)
	}
	return specScenes(stored)
}

// withTrailers gives the candidates of breakAt a trailer: "soon" (a movie), "unrendered"
// (a strip) and "later" (reruns) get scenes; only "soon" has its trailer rendered.
func withTrailers(in BreakInput) BreakInput {
	in.Candidates = slices.Clone(in.Candidates)
	for i := range in.Candidates {
		switch in.Candidates[i].TitleID {
		case "soon":
			in.Candidates[i].Trailer, in.Candidates[i].TrailerFrames = trailerScenes(3), 1900
		case "later", "unrendered":
			in.Candidates[i].Trailer = trailerScenes(4)
		}
	}
	return in
}

func count(c Composition, kind string) int {
	n := 0
	for _, k := range kinds(c) {
		if k == kind {
			n++
		}
	}
	return n
}

// The long promo (D43): the best candidate that has a rendered trailer and room for it airs
// it, once, ahead of the other promos; its sound is the full trailer and the end card makes none.
func TestComposeTrailer(t *testing.T) {
	end := time.Date(2026, 10, 14, 18, 25, 0, 0, prague)
	in := withTrailers(breakAt(end, 5*time.Minute, FillStrip))
	c, err := Compose(in)
	if err != nil {
		t.Fatal(err)
	}
	checkComposition(t, in, c)
	got := kinds(c)
	i := slices.Index(got, ClipTrailerBody)
	if i < 0 || got[i+1] != ClipTrailerEnd {
		t.Fatalf("no trailer with its end card in %v", got)
	}
	body, endCard := c.Parts[i], c.Parts[i+1]
	if body.TitleID != "soon" || body.Frames != 1900 || endCard.Frames != endCardFrames || endCard.Spec.Part != "end" {
		t.Errorf("trailer of %s, %d frames; end card %d frames, part %q", body.TitleID, body.Frames, endCard.Frames, endCard.Spec.Part)
	}
	// the long promo comes first among the promos, and the title is promoted once
	for _, p := range c.Parts[:i] {
		switch p.Kind {
		case ClipPromoBody, ClipShortPromo, ClipStripPromo:
			t.Errorf("%s ahead of the trailer in %v", p.Kind, got)
		}
	}
	for _, p := range c.Parts {
		if p.TitleID == "soon" && p.Kind != ClipTrailerBody && p.Kind != ClipTrailerEnd {
			t.Errorf("the trailer's title is promoted again: %s", p.Kind)
		}
	}
	if !slices.Equal(c.Trailers, []string{"soon"}) || !slices.Contains(c.Promos, "soon") {
		t.Errorf("trailers %v promos %v", c.Trailers, c.Promos)
	}
	// the body's sound is the full spec with the real airing; the end card plays no sound of its own
	if body.Sound == nil || body.Sound.Part != "full" || body.Sound.Airing == nil || body.Sound.Airing.Time != in.Candidates[0].Airing.Time {
		t.Errorf("sound part %q", body.Sound.Part)
	}
	if body.Spec.Part != "body" || body.Spec.Airing != nil || body.Spec.Mood != "cinematic" || len(body.Spec.Scenes) != 3 {
		t.Errorf("body spec: part %q, %d scenes", body.Spec.Part, len(body.Spec.Scenes))
	}
	items := 0
	for _, it := range c.Sound.Items {
		if it.Spec.Template == "trailer" {
			items++
			if it.Spec.Part != "full" || it.At != frameSeconds(body.At) {
				t.Errorf("sound item at %v, part %q", it.At, it.Spec.Part)
			}
		}
	}
	if items != 1 {
		t.Errorf("%d trailer sound items", items)
	}
	// its sound is a promo's: the airing is the body's in the key, whenever it airs
	a, b := *body.Sound, *body.Sound
	other := SpecAiring{Eyebrow: "Tomorrow", Time: "21:00"}
	b.Airing = &other
	ka, _ := soundKey(1, a)
	kb, _ := soundKey(1, b)
	if ka != kb {
		t.Error("a trailer's sound depends on the airing")
	}
	if item, ok := itemSpec(ClipTrailerBody, body.Spec); !ok || item.Part != "full" || item.Airing.Time != bodyAiring.Time {
		t.Errorf("item spec part %q, ok %v", item.Part, ok)
	}
	if _, ok := itemSpec(ClipTrailerEnd, endCard.Spec); ok {
		t.Error("the end card has a sound of its own")
	}
}

func TestComposeTrailerRules(t *testing.T) {
	end := time.Date(2026, 10, 14, 18, 25, 0, 0, prague)
	long := func() BreakInput { return withTrailers(breakAt(end, 5*time.Minute, FillStrip)) }
	hasTrailer := func(in BreakInput) bool {
		c, err := Compose(in)
		if err != nil {
			t.Fatal(err)
		}
		checkComposition(t, in, c)
		return count(c, ClipTrailerBody) > 0
	}

	if !hasTrailer(long()) {
		t.Fatal("no trailer in a five-minute break")
	}
	// not twice in a row: the title's trailer aired just before or after
	in := long()
	in.RecentTrailers = map[string]bool{"soon": true}
	if hasTrailer(in) {
		t.Error("a trailer again within the rest")
	}
	// rendered only: a trailer asked for but not rendered yet is not aired
	in = long()
	for i := range in.Candidates {
		in.Candidates[i].TrailerFrames = 0
	}
	if hasTrailer(in) {
		t.Error("an unrendered trailer was aired")
	}
	// what does not fit gives way to the promos: a minute is too short for 1900 frames
	if hasTrailer(withTrailers(breakAt(end, 70*time.Second, FillStrip))) {
		t.Error("a trailer in a break too short for it")
	}
	// its room is what is left beside the squeeze, ident and up next: just enough, then just short
	var fits, short int
	for frames := 2480; frames < 2620; frames++ {
		in := withTrailers(breakAt(end, time.Duration(frames)*time.Second/FPS, FillStrip))
		if hasTrailer(in) {
			if fits == 0 {
				fits = frames
			}
		} else {
			short = frames
		}
	}
	// the squeeze, the short ident, the shortest up next, the trailer (1900) and its end card
	want := squeezeIdentFrames + identShortFrames + upNextSpan.min + 1900 + endCardFrames
	if fits != want || short != want-1 {
		t.Errorf("a trailer fits from %d frames, not at %d; want from %d", fits, short, want)
	}

	// a strip or a marathon only in a long break
	strip := func(length time.Duration) BreakInput {
		in := withTrailers(breakAt(end, length, FillStrip))
		for i := range in.Candidates {
			if in.Candidates[i].TitleID == "soon" {
				in.Candidates[i].Fill = FillStrip
				in.Candidates[i].Airing = airingPhrase(in.Candidates[i].Airs, in.Start, prague, FillStrip)
			}
		}
		return in
	}
	if hasTrailer(strip(5 * time.Minute)) {
		t.Error("a strip's trailer in a short break")
	}
	if !hasTrailer(strip(9 * time.Minute)) {
		t.Error("no trailer for a strip in a long break")
	}

	// the title of the next program is promoted by up next, with no trailer
	in = long()
	in.Next.TitleID = "soon"
	if hasTrailer(in) {
		t.Error("the next program's trailer")
	}

	// at most one a break, the strongest first: two movies with trailers
	in = long()
	for i := range in.Candidates {
		if in.Candidates[i].TitleID == "later" {
			in.Candidates[i].Fill, in.Candidates[i].TrailerFrames = FillPremiere, 1800
			in.Candidates[i].Airing = airingPhrase(in.Candidates[i].Airs, in.Start, prague, FillPremiere)
		}
	}
	c, err := Compose(in)
	if err != nil {
		t.Fatal(err)
	}
	if count(c, ClipTrailerBody) != 1 || c.Trailers[0] != "later" {
		t.Errorf("two trailers or not the premiere's: %v %v", kinds(c), c.Trailers)
	}
}

// Every gap is still filled to the frame, and the specs still valid, with trailers about, in a
// break with an up next and in one without a next program (nothing flexible to take up what a
// trailer leaves, so it may not leave a sliver).
func TestComposeTrailerFillsEveryGap(t *testing.T) {
	end := time.Date(2026, 10, 14, 20, 15, 0, 0, prague)
	trailers := 0
	for _, hasNext := range []bool{true, false} {
		for s := 60; s <= 15*60; s += 13 {
			for _, fill := range []string{FillStrip, FillMovie} {
				in := withTrailers(breakAt(end, time.Duration(s)*time.Second+40*time.Millisecond*time.Duration(s%25), fill))
				if !hasNext {
					in.Next, in.Later = nil, nil
				}
				c, err := Compose(in)
				if err != nil {
					t.Fatalf("%ds, next program %v: %v", s, hasNext, err)
				}
				checkComposition(t, in, c)
				trailers += count(c, ClipTrailerBody)
				if count(c, ClipTrailerBody) != count(c, ClipTrailerEnd) || count(c, ClipTrailerBody) > 1 {
					t.Fatalf("%ds, next program %v: %v", s, hasNext, kinds(c))
				}
			}
		}
	}
	if trailers == 0 {
		t.Error("no trailer in any break")
	}
}

// With no next program the break has no up next to stretch, so a trailer that would leave a
// stretch no sting can close (under 5 s) gives way: the break takes the other promos instead of
// failing and falling over to the evergreen one.
func TestComposeTrailerLeavesNoSliver(t *testing.T) {
	end := time.Date(2026, 10, 14, 20, 15, 0, 0, prague)
	// 1989 frames less the short ident's 130 leave 1859 for a trailer of 1700 and its end card of
	// 100: 59 frames, which stings cannot close
	in := breakAt(end, 1989*time.Second/FPS, FillStrip)
	in.Next, in.Later = nil, nil
	soon := in.Candidates[0]
	soon.Trailer, soon.TrailerFrames = trailerScenes(3), 1700
	in.Candidates = []Candidate{soon}
	c, err := Compose(in)
	if err != nil {
		t.Fatalf("a trailer left a sliver: %v", err)
	}
	checkComposition(t, in, c)
	if count(c, ClipTrailerBody) != 0 {
		t.Errorf("the trailer aired and left %d frames: %v", 1989-130-1800, kinds(c))
	}
	// a few frames more and the leftover is a sting's: the trailer airs
	in.Frames += 70
	c, err = Compose(in)
	if err != nil {
		t.Fatal(err)
	}
	checkComposition(t, in, c)
	if count(c, ClipTrailerBody) != 1 {
		t.Errorf("no trailer when a sting closes the rest: %v", kinds(c))
	}
}

// A bulletin's slot holds the promos and stings that air when no bulletin can: never a
// trailer, which would not fit a slot of 15 to 60 s anyway.
func TestTrailerNeverFillsASlot(t *testing.T) {
	end := time.Date(2026, 10, 14, 20, 0, 0, 0, prague)
	in := withTrailers(breakAt(end, 12*time.Minute, FillStrip))
	in.Bulletins = []string{"news", "weather"}
	c, err := Compose(in)
	if err != nil {
		t.Fatal(err)
	}
	checkComposition(t, in, c)
	if len(c.Slots) == 0 {
		t.Fatalf("no slots in %v", kinds(c))
	}
	for _, p := range c.Parts {
		if p.Slot != "" && (p.Kind == ClipTrailerBody || p.Kind == ClipTrailerEnd) {
			t.Errorf("a trailer in the slot of a %s", p.Slot)
		}
	}
	if count(c, ClipTrailerBody) != 1 {
		t.Errorf("%d trailers beside the slots", count(c, ClipTrailerBody))
	}
}

func TestSpecScenes(t *testing.T) {
	scenes := trailerScenes(3)
	if len(scenes) != 3 {
		t.Fatal("scenes")
	}
	first, second := scenes[0], scenes[1]
	if len(first.Frames) != 25 || !strings.HasSuffix(first.Frames[24], "/24.jpg") || !strings.Contains(first.Frames[0], "/600000_25/") {
		t.Errorf("frames %d, first %s", len(first.Frames), first.Frames[0])
	}
	// a stream of its own for an original-language track, the default form otherwise
	if !strings.Contains(first.Sound.URL, "/1/600000_1000.flac") || strings.Contains(second.Sound.URL, "/1/") || !strings.HasSuffix(second.Sound.URL, "/900000_1000.flac") {
		t.Errorf("sounds %s and %s", first.Sound.URL, second.Sound.URL)
	}
	if first.Sound.In != 0 || first.Sound.Out != 1 {
		t.Errorf("sound %+v, want 0 to 1 s", first.Sound)
	}
	// lines closer than a third of a second are one stretch; none is a list, not null
	if len(first.Lines) != 2 || first.Lines[0] != [2]float64{0.1, 0.4} || first.Lines[1] != [2]float64{0.8, 0.95} {
		t.Errorf("lines %v", first.Lines)
	}
	if got := specLines([]CouchTvTrailerLine{{FromMs: 500, ToMs: 1500}, {FromMs: 1700, ToMs: 2500}}); len(got) != 1 || got[0] != [2]float64{0.5, 2.5} {
		t.Errorf("joined lines %v", got)
	}
	if got := specLines(nil); got == nil || len(got) != 0 {
		t.Errorf("no lines %#v", got)
	}
	if specScenes(make([]CouchTvTrailerScene, 2)) != nil {
		t.Error("two scenes make a trailer")
	}
}
