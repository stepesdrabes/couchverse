package couchtv

import (
	"fmt"
	"slices"
	"testing"
	"time"
)

// checkSlots asserts what a break with bulletin slots keeps: each slot exactly its kind's
// length, made of whole parts that hold it (promos and stings), after the promos and the
// lineup and right before up next.
func checkSlots(t *testing.T, in BreakInput, c Composition, want []string) {
	t.Helper()
	checkComposition(t, in, c)
	var got []string
	for _, s := range c.Slots {
		got = append(got, s.Kind)
		if s.Frames != bulletinFrames[s.Kind] {
			t.Errorf("the %s slot is %d frames, want %d", s.Kind, s.Frames, bulletinFrames[s.Kind])
		}
	}
	if !slices.Equal(got, want) {
		t.Fatalf("slots %v, want %v", got, want)
	}
	if len(c.Slots) == 0 {
		return
	}
	last := c.Slots[len(c.Slots)-1]
	upNext := c.Parts[len(c.Parts)-1]
	if upNext.Kind != ClipUpNext || last.At+last.Frames != upNext.At {
		t.Errorf("the slots do not end where up next starts: %+v, %+v", c.Slots, upNext)
	}
	for i := 1; i < len(c.Slots); i++ {
		if c.Slots[i].At != c.Slots[i-1].At+c.Slots[i-1].Frames {
			t.Errorf("the slots are apart: %+v", c.Slots)
		}
	}
	for _, p := range c.Parts {
		inSlot := p.At >= c.Slots[0].At && p.At < last.At+last.Frames
		if inSlot != (p.Slot != "") {
			t.Errorf("%s at %d: slot %q", p.Kind, p.At, p.Slot)
		}
		if inSlot && p.Kind != ClipSting && p.Kind != ClipShortPromo && p.Kind != ClipStripPromo && p.Kind != ClipPromoBody && p.Kind != ClipPromoEnd {
			t.Errorf("a slot holds a %s", p.Kind)
		}
		if !inSlot && p.At > c.Slots[0].At && p.Kind != ClipUpNext {
			t.Errorf("a %s after the slots", p.Kind)
		}
	}
}

func TestComposeBulletinSlots(t *testing.T) {
	end := time.Date(2026, 10, 14, 18, 0, 0, 0, prague)
	tests := []struct {
		name      string
		length    time.Duration
		bulletins []string
		want      []string
	}{
		{"the news before up next", 3 * time.Minute, []string{BulletinNews}, []string{BulletinNews}},
		{"news, then the weather", 4 * time.Minute, []string{BulletinNews, BulletinWeather}, []string{BulletinNews, BulletinWeather}},
		{"the weather alone", 2 * time.Minute, []string{BulletinWeather}, []string{BulletinWeather}},
		// 100 s: the news (54.4 s) and the rest of the break fit, the weather (38.4 s) too would not
		{"too short for both", 100 * time.Second, []string{BulletinNews, BulletinWeather}, []string{BulletinNews}},
		{"too short for any", 80 * time.Second, []string{BulletinNews}, nil},
	}
	for _, tt := range tests {
		in := breakAt(end, tt.length, FillStrip)
		in.Bulletins = tt.bulletins
		c, err := Compose(in)
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		t.Run(tt.name, func(t *testing.T) { checkSlots(t, in, c, tt.want) })
	}

	// the fallback promotes what the rest of the break does not, the lineup stays before it
	in := breakAt(end, 105*time.Second, FillStrip)
	in.Bulletins = []string{BulletinNews}
	c, _ := Compose(in)
	var outside, inside []string
	lineup := -1
	for i, p := range c.Parts {
		switch {
		case p.Kind == ClipLineup:
			lineup = i
		case p.TitleID != "" && p.Slot != "":
			inside = append(inside, p.TitleID)
		case p.TitleID != "":
			outside = append(outside, p.TitleID)
		}
	}
	if lineup < 0 || c.Parts[lineup+1].Slot == "" {
		t.Errorf("the lineup is not right before the slot: %v", kinds(c))
	}
	if len(inside) == 0 || slices.Contains(outside, inside[0]) {
		t.Errorf("the fallback promotes %v beside %v", inside, outside)
	}
}

// Every gap from the shortest that keeps a slot up to half an hour is filled to the frame with
// its slots whole, and none is kept where they would crowd out the break.
func TestComposeBulletinSlotsFillEveryGap(t *testing.T) {
	end := time.Date(2026, 10, 14, 7, 0, 0, 0, prague)
	both := []string{BulletinNews, BulletinWeather}
	var lengths []int
	for f := 600; f <= 4000; f += 7 {
		lengths = append(lengths, f)
	}
	// every frame where a slot comes or goes
	for _, edge := range []int{slotRestFrames + newsFrames, slotRestFrames + newsFrames + weatherFrames} {
		for f := edge - 30; f <= edge+30; f++ {
			lengths = append(lengths, f)
		}
	}
	for f := 4000; f <= 45000; f += 997 {
		lengths = append(lengths, f)
	}
	for _, f := range lengths {
		in := breakAt(end, time.Duration(f)*time.Second/FPS, FillStrip)
		in.Bulletins = both
		c, err := Compose(in)
		if err != nil {
			t.Fatalf("%d frames: %v", f, err)
		}
		var want []string
		switch {
		case f >= slotRestFrames+newsFrames+weatherFrames:
			want = both
		case f >= slotRestFrames+newsFrames:
			want = both[:1]
		}
		checkSlots(t, in, c, want)
	}
}

// A slot's fallback is exactly its length whatever the promos: a promo leaves nothing or a
// stretch stings can close, each title once.
func TestSlotFallback(t *testing.T) {
	stills := []Shot{{Kind: "still", Image: "backdrop", From: &Crop{0, 0, 1, 1}}}
	body := func(id string, frames int) Candidate {
		return Candidate{TitleID: id, Title: specTitle(id), Mood: "bright", Shots: stills, BodyFrames: frames,
			Airing: SpecAiring{Eyebrow: "Tonight", Time: "20:15"}}
	}
	check := func(name string, pieces []piece, frames int) (kinds []string) {
		total := 0
		for _, p := range pieces {
			kinds = append(kinds, p.part.Kind)
			total += p.part.Frames
			if p.part.Kind == ClipSting && (p.part.Frames < stingSpan.min || p.part.Frames > stingSpan.max) {
				t.Errorf("%s: a sting of %d frames", name, p.part.Frames)
			}
		}
		if total != frames {
			t.Errorf("%s: %v for %d frames, want %d", name, kinds, total, frames)
		}
		return kinds
	}
	for _, c := range []struct {
		name   string
		frames int
		cands  []Candidate
		want   []string
	}{
		{"a full promo and a sting", 500, []Candidate{body("a", 250)}, []string{ClipPromoBody, ClipPromoEnd, ClipSting}},
		// 480 frames of promo would leave 20: the short promo instead
		{"no sliver after a promo", 500, []Candidate{body("a", 380)}, []string{ClipShortPromo, ClipSting, ClipSting}},
		{"two short promos and a sting", 600, []Candidate{body("a", 0), body("b", 0)}, []string{ClipShortPromo, ClipShortPromo, ClipSting}},
		{"stings alone", 500, nil, []string{ClipSting, ClipSting}},
	} {
		got := check(c.name, slotFallback(c.cands, map[string]bool{}, c.frames), c.frames)
		if !slices.Equal(got, c.want) {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}

	// every slot from the shortest sting up to a minute and a half, whatever the promos and
	// their lengths (a trailer of a minute and a half among them)
	sets := map[string][]Candidate{
		"none":    nil,
		"a body":  {body("a", 415)},
		"two":     {body("a", 330), body("b", 0)},
		"trailer": {body("a", 2250), body("b", 415)},
	}
	for frames := stingSpan.min; frames <= 90*FPS; frames++ {
		for name, cands := range sets {
			check(fmt.Sprintf("%s in %d frames", name, frames), slotFallback(cands, map[string]bool{}, frames), frames)
		}
	}

	// and each kind's real length, whatever there is to promote
	for kind, frames := range bulletinFrames {
		for n := 0; n <= 4; n++ {
			var cands []Candidate
			for i := 0; i < n; i++ {
				cands = append(cands, body(fmt.Sprint(i), 330+25*i))
			}
			total := 0
			for _, p := range slotFallback(cands, map[string]bool{}, frames) {
				total += p.part.Frames
			}
			if total != frames {
				t.Errorf("the %s slot's fallback of %d promos is %d frames, want %d", kind, n, total, frames)
			}
		}
	}
}

func TestBulletinKinds(t *testing.T) {
	at := func(day, hour, minute int) time.Time { return time.Date(2026, 10, day, hour, minute, 0, 0, prague) }
	for _, c := range []struct {
		name            string
		start, end, ago time.Time
		next            string
		want            []string
	}{
		{"the news on the hour", at(14, 9, 56), at(14, 10, 0), at(14, 9, 30), FillReruns, []string{BulletinNews}},
		{"no news off the hour", at(14, 10, 21), at(14, 10, 25), at(14, 10, 0), FillReruns, nil},
		{"the first news after the morning's first fetch", at(14, 5, 55), at(14, 6, 0), at(14, 5, 30), FillReruns, []string{BulletinNews}},
		// a break starting 05:44 cannot have the 05:30 fetch rendered in time; the night's
		// last (00:30) is too old by 06:00
		{"too early for a fetch to be rendered", at(14, 5, 44), at(14, 6, 0), at(14, 5, 30), FillReruns, nil},
		{"the news at midnight", at(14, 23, 56), at(15, 0, 0), at(14, 23, 30), FillReruns, []string{BulletinNews}},
		{"none in the night", at(15, 0, 56), at(15, 1, 0), at(15, 0, 30), FillReruns, nil},
		{"no news on the odd hours", at(14, 8, 56), at(14, 9, 0), at(14, 8, 30), FillReruns, nil},
		{"the morning weather without the news", at(14, 6, 55), at(14, 7, 0), at(14, 6, 30), FillStrip, []string{BulletinWeather}},
		{"news and the weather at noon", at(14, 11, 55), at(14, 12, 0), at(14, 11, 30), FillStrip, []string{BulletinNews, BulletinWeather}},
		{"the weather in the first break after noon", at(14, 12, 10), at(14, 12, 15), at(14, 11, 50), FillReruns, []string{BulletinWeather}},
		{"not in the break after it", at(14, 12, 35), at(14, 12, 40), at(14, 12, 15), FillReruns, nil},
		{"not an hour late", at(14, 13, 5), at(14, 13, 10), at(14, 11, 0), FillMovie, nil},
		{"the weather before the evening movie", at(14, 20, 10), at(14, 20, 15), at(14, 19, 45), FillMovie, []string{BulletinWeather}},
		{"not before the late movie", at(14, 22, 25), at(14, 22, 30), at(14, 21, 40), FillMovie, nil},
	} {
		if got := bulletinKinds(c.start, c.end, c.ago, c.next, prague); !slices.Equal(got, c.want) {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

func TestLastBulletin(t *testing.T) {
	at := func(day, hour, minute int) time.Time { return time.Date(2026, 10, day, hour, minute, 0, 0, prague) }
	for _, c := range []struct {
		at    time.Time
		every time.Duration
		want  time.Time
	}{
		{at(14, 17, 45), 20 * time.Minute, at(14, 17, 30)},
		{at(14, 17, 30), 20 * time.Minute, at(14, 17, 30)},
		{at(14, 5, 40), 20 * time.Minute, at(14, 5, 30)},
		{at(14, 5, 20), 20 * time.Minute, at(14, 0, 30)},
		{at(15, 0, 40), time.Hour, at(15, 0, 30)},
		{at(14, 16, 59), time.Hour, at(14, 16, 30)},
	} {
		if got := lastBulletin(c.at, prague, c.every); !got.Equal(c.want) {
			t.Errorf("at %s every %s: %s, want %s", c.at, c.every, got.In(prague), c.want)
		}
		// the grid nextBulletin walks
		if next := nextBulletin(lastBulletin(c.at, prague, c.every), prague, c.every); !next.After(c.at) {
			t.Errorf("at %s every %s: the next run %s is not after it", c.at, c.every, next)
		}
	}
}

// slotsOverAWeek counts, over the breaks of a planned week, the bulletins D37 asks for and the
// ones a break has room to keep, by kind.
func slotsOverAWeek(rows []Airing) (want, kept map[string]int) {
	want, kept = map[string]int{}, map[string]int{}
	for i, a := range rows {
		if a.Kind != KindBreak || i == 0 || i == len(rows)-1 {
			continue
		}
		var prevStart time.Time
		if prev := rows[i-1]; prev.Kind == KindProgram {
			prevStart = prev.StartsAt
		}
		kinds := bulletinKinds(a.StartsAt, a.EndsAt, prevStart, rows[i+1].Fill, prague)
		in := BreakInput{Frames: framesOf(a.EndsAt.Sub(a.StartsAt).Seconds()), Next: &BreakProgram{}, Bulletins: kinds}
		got, _ := keptSlots(in)
		for _, k := range kinds {
			want[k]++
		}
		for _, k := range got {
			kept[k]++
		}
	}
	return want, kept
}

// The slots are a minute and half a minute long, and a break keeps one only where it still has
// the squeeze, the short ident, a short promo and up next beside it: over a planned week of the
// production library's shape nearly every slot the schedule asks for still has its room.
func TestBulletinSlotsOverAWeek(t *testing.T) {
	rows := planWeek(ownerLibrary().Library, "2026-10-12", 7)
	want, kept := slotsOverAWeek(rows)
	t.Logf("wanted %v, kept %v", want, kept)
	if want[BulletinNews] < 10 || want[BulletinWeather] < 10 {
		t.Fatalf("the week asks for %v: too few slots to judge", want)
	}
	for _, kind := range []string{BulletinNews, BulletinWeather} {
		if got := float64(kept[kind]) / float64(want[kind]); got < 0.8 {
			t.Errorf("%s: %d of %d slots kept (%.0f%%), want at least 80%%", kind, kept[kind], want[kind], 100*got)
		}
	}
}
