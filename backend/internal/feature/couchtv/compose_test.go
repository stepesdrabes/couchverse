package couchtv

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
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
		{TitleID: "strip", Title: specTitle("strip"), Airs: start.Add(20 * time.Hour), Fill: FillStrip, StripMood: "bright",
			StripFoot: &StripFoot{Kind: "count", Count: 2}},
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
		{time.Date(2026, 10, 18, 20, 15, 0, 0, prague), FillSaga, SpecAiring{Eyebrow: "Saga", Day: "Sunday", Time: "20:15"}},
		{time.Date(2026, 10, 17, 14, 0, 0, 0, prague), FillMarathon, SpecAiring{Eyebrow: "Marathon", Day: "Saturday", Time: "14:00"}},
		{time.Date(2026, 10, 18, 9, 30, 0, 0, prague), FillReruns, SpecAiring{Eyebrow: "This week", Day: "Sunday", Time: "09:30"}},
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

// Every screen draws a title's logo, and every one that shows a title in motion its scenes: up
// next, the squeeze's next card, the lineup, the strip, short and full promos and the end
// cards; the squeeze's ending is the program itself. The screens that render for every break
// draw the backdrop's smaller copy, the promo body (rendered once per title) its original, and
// the body's sound is keyed as its clip keeps it.
func TestComposeTitlesLogosAndScenes(t *testing.T) {
	in := breakAt(time.Date(2026, 10, 14, 18, 0, 0, 0, prague), 90*time.Second, FillStrip)
	dress := func(st *SpecTitle) {
		st.Logo = &SpecLogo{URL: "/media/v1/artwork/" + st.Name + "-logo?v=1&size=1600x800", Aspect: 3.2}
		st.Footage = []Shot{{Kind: "footage", Frames: FramesURLs("f-"+st.Name, time.Minute, 50)}}
	}
	dress(&in.Ending.Title)
	dress(&in.Next.Title)
	for i := range in.Later {
		dress(&in.Later[i].Title)
	}
	for i := range in.Candidates {
		dress(&in.Candidates[i].Title)
		in.Candidates[i].BodyBackdrop = "/media/v1/artwork/" + in.Candidates[i].TitleID + "-original"
	}
	c, err := Compose(in)
	if err != nil {
		t.Fatal(err)
	}
	checkComposition(t, in, c)
	seen := map[string]bool{}
	for _, p := range c.Parts {
		seen[p.Kind] = true
		var moving []*SpecTitle
		switch p.Kind {
		case ClipSqueeze:
			if p.Spec.Ending.Footage != nil || p.Spec.Ending.Logo == nil {
				t.Errorf("the squeeze's ending: %+v", p.Spec.Ending)
			}
			moving = append(moving, &p.Spec.Next.Title)
		case ClipLineup:
			for i := range p.Spec.Items {
				moving = append(moving, &p.Spec.Items[i].Title)
			}
		case ClipUpNext, ClipStripPromo, ClipShortPromo, ClipPromoBody, ClipPromoEnd:
			moving = append(moving, p.Spec.Title)
		}
		for _, st := range moving {
			if st.Logo == nil || len(st.Footage) != 1 || st.Footage[0].Frames[0] != "/media/v1/frames/f-"+st.Name+"/60000_50/0.jpg" {
				t.Errorf("%s shows %s with logo %v and %d scenes", p.Kind, st.Name, st.Logo, len(st.Footage))
			}
		}
		switch p.Kind {
		case ClipPromoBody:
			if p.Spec.Title.Backdrop != "/media/v1/artwork/"+p.TitleID+"-original" || len(p.Spec.Shots) == 0 {
				t.Errorf("the body of %s: backdrop %s, %d shots", p.TitleID, p.Spec.Title.Backdrop, len(p.Spec.Shots))
			}
			clip, _ := itemSpec(p.Kind, p.Spec)
			kept, _ := soundKey(1, clip)
			mixed, _ := soundKey(1, *p.Sound)
			if kept != mixed {
				t.Errorf("the body of %s sounds as %+v in the break, its clip keeps %+v", p.TitleID, *p.Sound, clip)
			}
		case ClipPromoEnd, ClipShortPromo, ClipStripPromo, ClipUpNext:
			if p.Spec.Title.Backdrop != "/media/v1/artwork/"+p.Spec.Title.Name {
				t.Errorf("%s draws the backdrop %s", p.Kind, p.Spec.Title.Backdrop)
			}
		}
	}
	for _, kind := range []string{ClipSqueeze, ClipUpNext, ClipLineup, ClipStripPromo, ClipShortPromo, ClipPromoBody, ClipPromoEnd} {
		if !seen[kind] {
			t.Errorf("no %s composed: %v", kind, kinds(c))
		}
	}
}

// A title's footage is its analysed shots, the frame URLs its promo body has, and none without
// analysis; its logo goes along as the store read it.
func TestTitleFootageAndLogo(t *testing.T) {
	f := titleFacts{Name: "Iron Man", Logo: &SpecLogo{URL: "/media/v1/artwork/l?v=1&size=1600x800", Aspect: 3.994},
		Shots: []analyzedShot{{MediaFileID: "file-1", StartMs: 600000, DurationMs: 2000}, {MediaFileID: "file-1", StartMs: 900040, DurationMs: 1240}}}
	st := f.spec()
	if !reflect.DeepEqual(st.Footage, f.shots()) || len(st.Footage) != 2 || len(st.Footage[1].Frames) != 31 ||
		st.Footage[1].Frames[30] != "/media/v1/frames/file-1/900040_31/30.jpg" {
		t.Errorf("footage %+v", st.Footage)
	}
	if st.Logo == nil || *st.Logo != *f.Logo || st.Logo == f.Logo {
		t.Errorf("logo %+v", st.Logo)
	}
	bare := titleFacts{Name: "Bare"}
	data, _ := json.Marshal(bare.spec())
	if strings.Contains(string(data), "footage") || strings.Contains(string(data), "logo") {
		t.Errorf("a title without analysis or logo: %s", data)
	}
	if shots := bare.shots(); len(shots) == 0 || shots[0].Kind != "still" {
		t.Errorf("its promo body shows %+v", shots)
	}
}

// A strip promo's foot counts the episodes "in order", which a series airing at random
// does not keep, so its promo has none.
func TestStripPromoFoot(t *testing.T) {
	r := &composition{cfg: DefaultConfig(), titles: map[string]titleFacts{
		"ordered": {Name: "Ordered"}, "random": {Name: "Random", Random: true},
	}}
	from := local("2026-10-14", "12:00")
	for _, c := range []struct {
		title, slot string
		want        *StripFoot
	}{
		{"ordered", "sitcom-strip", &StripFoot{Kind: "count", Count: 2}},
		{"random", "sitcom-strip", nil},
		{"ordered", "early-strip", nil}, // one episode a day, nothing to count
	} {
		p := program{Airing: Airing{StartsAt: local("2026-10-14", "18:00"), Fill: FillStrip, Slot: c.slot}}
		got := r.candidate(c.title, p, from)
		if got.Strip == nil || fmt.Sprint(got.StripFoot) != fmt.Sprint(c.want) {
			t.Errorf("%s in %s: strip %v, foot %+v, want %+v", c.title, c.slot, got.Strip, got.StripFoot, c.want)
		}
		grp, _ := promoPieces(got, stripPromoFrames)
		if len(grp) != 1 || grp[0].part.Kind != ClipStripPromo || fmt.Sprint(grp[0].part.Spec.Foot) != fmt.Sprint(c.want) {
			t.Errorf("%s in %s: the strip promo's foot is %+v", c.title, c.slot, grp)
		}
	}
}

// fullDesigns are the designs of a break's full idents, in order.
func fullDesigns(c Composition) []string {
	var out []string
	for _, p := range c.Parts {
		if p.Kind == ClipIdentFull {
			out = append(out, p.Spec.Design)
		}
		if p.Kind == ClipIdentShort && p.Spec.Design != "" {
			out = append(out, "short:"+p.Spec.Design)
		}
	}
	return out
}

// The full ident's designs take turns by broadcast day (D35): the prime-time ident is the
// day's, the full idents a long gap repeats in the same break take the designs after it.
func TestIdentDesigns(t *testing.T) {
	next := func(d string) string {
		return identDesigns[(slices.Index(identDesigns, d)+1)%len(identDesigns)]
	}
	// a week of prime-time breaks: each day the next design, all of them within four days
	seen := map[string]bool{}
	prev := ""
	for d := range 7 {
		in := breakAt(time.Date(2026, 10, 14+d, 20, 15, 0, 0, prague), time.Minute, FillMovie)
		in.Day = time.Date(2026, 10, 14+d, 0, 0, 0, 0, prague)
		c, err := Compose(in)
		if err != nil {
			t.Fatal(err)
		}
		checkComposition(t, in, c)
		got := fullDesigns(c)
		if len(got) != 1 || !slices.Contains(identDesigns, got[0]) {
			t.Fatalf("day %d: full idents %v", d, got)
		}
		if prev != "" && got[0] != next(prev) {
			t.Errorf("day %d: %s after %s, want %s", d, got[0], prev, next(prev))
		}
		if d < len(identDesigns) {
			seen[got[0]] = true
		}
		prev = got[0]
	}
	if len(seen) != len(identDesigns) {
		t.Errorf("the first %d days air %v", len(identDesigns), seen)
	}

	// a long gap with nothing to promote repeats the full ident in the designs that follow,
	// after the prime-time one or on its own
	day := time.Date(2026, 10, 16, 0, 0, 0, 0, prague)
	first := identDesign(day, 0)
	for _, tt := range []struct {
		name string
		end  time.Time
		fill string
	}{
		{"before the movie", time.Date(2026, 10, 16, 20, 15, 0, 0, prague), FillMovie},
		{"between reruns", time.Date(2026, 10, 16, 15, 25, 0, 0, prague), FillReruns},
	} {
		in := breakAt(tt.end, 3*time.Minute, tt.fill)
		in.Day, in.Candidates = day, nil
		c, err := Compose(in)
		if err != nil {
			t.Fatal(err)
		}
		checkComposition(t, in, c)
		got := fullDesigns(c)
		if len(got) < len(identDesigns)+1 || got[0] != first {
			t.Fatalf("%s: full idents %v, want at least %d from %s", tt.name, got, len(identDesigns)+1, first)
		}
		for i := 1; i < len(got); i++ {
			if got[i] != next(got[i-1]) {
				t.Errorf("%s: %v does not take turns", tt.name, got)
				break
			}
		}
	}

	// a break after midnight is the broadcast day before it
	cfg := DefaultConfig()
	if a, b := identDesign(cfg.dayOf(local("2026-10-17", "00:40")), 0), identDesign(cfg.dayOf(local("2026-10-16", "20:15")), 0); a != b {
		t.Errorf("00:40 airs %s, the evening before %s", a, b)
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
	// one design whatever the day, so the evergreen break is composed once
	if got := fullDesigns(c); !slices.Equal(got, []string{evergreenDesign}) {
		t.Errorf("full idents %v, want %s", got, evergreenDesign)
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

// A render waits for the quiet hours unless what it is for airs soon, and never starts
// later than renderSoon before it airs.
func TestRenderAt(t *testing.T) {
	prague, _ := time.LoadLocation("Europe/Prague")
	at := func(day, hour, minute int) time.Time { return time.Date(2026, 10, day, hour, minute, 0, 0, prague) }
	cases := []struct {
		name           string
		now, airs, run time.Time
	}{
		{"what airs any time renders at once", at(12, 14, 0), time.Time{}, at(12, 14, 0)},
		{"what airs within three hours too", at(12, 14, 0), at(12, 16, 59), at(12, 14, 0)},
		{"tomorrow evening waits for tonight's quiet hours", at(12, 14, 0), at(13, 20, 15), at(13, 1, 0)},
		{"in the quiet hours a render starts at once", at(13, 2, 30), at(13, 20, 15), at(13, 2, 30)},
		{"after them it waits for the next night", at(13, 9, 0), at(14, 18, 0), at(14, 1, 0)},
		{"before them on the same night", at(13, 0, 30), at(13, 18, 0), at(13, 1, 0)},
		{"never later than three hours before it airs", at(12, 21, 0), at(13, 2, 0), at(12, 23, 0)},
	}
	for _, c := range cases {
		if got := renderAt(c.now, c.airs, prague); !got.Equal(c.run) {
			t.Errorf("%s: %s, want %s", c.name, got.In(prague).Format(time.DateTime), c.run.Format(time.DateTime))
		}
	}
}
