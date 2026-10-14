package couchtv

import (
	"testing"
	"time"
)

// currentBodies holds the key of every body a break could ask for a title in the window: the
// promo's and the trailer's, each in the form of a premiere and of any other airing; a title
// without a trailer has none; a title whose facts changed has other keys.
func TestCurrentBodies(t *testing.T) {
	scenes := []CouchTvTrailerScene{
		{MediaFileID: "f", AudioFileID: "f", StartMs: 1000, DurationMs: 5000},
		{MediaFileID: "f", AudioFileID: "f", StartMs: 20000, DurationMs: 5000},
		{MediaFileID: "f", AudioFileID: "f", StartMs: 40000, DurationMs: 5000},
	}
	run := func(titles map[string]titleFacts) *composition {
		return &composition{Composer: &Composer{}, versions: map[string]int{"promo": 3, "trailer": 2}, now: time.Date(2026, 10, 12, 10, 0, 0, 0, prague), titles: titles}
	}
	r := run(map[string]titleFacts{"a": {Name: "A", Trailer: scenes}, "b": {Name: "B"}})
	keys, err := r.currentBodies()
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 6 {
		t.Errorf("%d keys, want a promo and a trailer in two forms for A and a promo in two for B", len(keys))
	}
	for id := range r.titles {
		for _, fill := range []string{FillMovie, FillPremiere, FillStrip, ""} {
			c := r.candidate(id, program{Airing: Airing{Fill: fill}}, r.now)
			promo, err := clipKey(3, promoBody(c))
			if err != nil || !keys[promo] {
				t.Errorf("title %s as %q: its promo body is not current (%v)", id, fill, err)
			}
			if len(c.Trailer) > 0 {
				trailer, err := clipKey(2, trailerBody(c))
				if err != nil || !keys[trailer] {
					t.Errorf("title %s as %q: its trailer body is not current (%v)", id, fill, err)
				}
			}
		}
	}
	premiere, plain := r.candidate("b", program{Airing: Airing{Fill: FillPremiere}}, r.now), r.candidate("b", program{}, r.now)
	if a, _ := clipKey(3, promoBody(premiere)); a == "" {
		t.Fatal("no key")
	} else if b, _ := clipKey(3, promoBody(plain)); a == b {
		t.Error("a premiere's promo body has the key of the other one")
	}

	changed, err := run(map[string]titleFacts{"a": {Name: "A", Tagline: "Now with a tagline", Trailer: scenes}, "b": {Name: "B"}}).currentBodies()
	if err != nil {
		t.Fatal(err)
	}
	shared := 0
	for k := range changed {
		if keys[k] {
			shared++
		}
	}
	if shared != 2 {
		t.Errorf("%d keys survive A's new tagline, want B's two", shared)
	}
}

// A body is asked for again whenever what it draws changes: the title's logo, backdrop, footage,
// bed or accents, its mood, whether it is a premiere, the scenes, the template's version.
func TestBodyKeysFollowWhatTheyDraw(t *testing.T) {
	scene := func(file string, start int) CouchTvTrailerScene {
		return CouchTvTrailerScene{MediaFileID: file, AudioFileID: file, StartMs: start, DurationMs: 5000}
	}
	base := titleFacts{Name: "A", Trailer: []CouchTvTrailerScene{scene("f", 1000), scene("f", 20000), scene("f", 40000)},
		Logo: &SpecLogo{URL: "/logo/1", Aspect: 2}, BodyBackdrop: "/art/1", Mood: "grit",
		Shots: []analyzedShot{{MediaFileID: "f", StartMs: 5000, DurationMs: 2000}}, Promo: &Bed{URL: "/bed/1", Out: 30},
		Accents: Accents{Backdrop: "#112233"}}
	keyOf := func(f titleFacts, fill string, version int) (promo, trailer string) {
		r := &composition{Composer: &Composer{}, versions: map[string]int{"promo": version, "trailer": version},
			now: time.Date(2026, 10, 12, 10, 0, 0, 0, prague), titles: map[string]titleFacts{"a": f}}
		c := r.candidate("a", program{Airing: Airing{Fill: fill}}, r.now)
		var err error
		if promo, err = clipKey(version, promoBody(c)); err != nil {
			t.Fatal(err)
		}
		if trailer, err = clipKey(version, trailerBody(c)); err != nil {
			t.Fatal(err)
		}
		return promo, trailer
	}
	promo, trailer := keyOf(base, FillMovie, 1)
	for name, change := range map[string]func(*titleFacts){
		"logo":     func(f *titleFacts) { f.Logo = &SpecLogo{URL: "/logo/2", Aspect: 2} },
		"backdrop": func(f *titleFacts) { f.BodyBackdrop = "/art/2" },
		"footage":  func(f *titleFacts) { f.Shots = []analyzedShot{{MediaFileID: "f", StartMs: 9000, DurationMs: 2000}} },
		"bed":      func(f *titleFacts) { f.Promo = &Bed{URL: "/bed/2", Out: 30} },
		"accents":  func(f *titleFacts) { f.Accents = Accents{Backdrop: "#445566"} },
		"mood":     func(f *titleFacts) { f.Mood = "bright" },
		"scenes": func(f *titleFacts) {
			f.Trailer = []CouchTvTrailerScene{scene("g", 1000), scene("f", 20000), scene("f", 40000)}
		},
	} {
		f := base
		change(&f)
		p, tr := keyOf(f, FillMovie, 1)
		if tr == trailer {
			t.Errorf("a new %s keeps the trailer's key", name)
		}
		if p == promo && name != "scenes" {
			t.Errorf("a new %s keeps the promo's key", name)
		}
	}
	if p, tr := keyOf(base, FillPremiere, 1); p == promo || tr == trailer {
		t.Error("a premiere keeps the keys of the other airings")
	}
	if p, tr := keyOf(base, FillMovie, 2); p == promo || tr == trailer {
		t.Error("a new template version keeps the keys")
	}
}

// A long render (a full ident, a trailer's body) starts only when it is over before the first
// bulletin of the day is asked for, half past five; one that is not waits for the next night; the
// renders that take ten minutes or less start when they could.
func TestLongRenderStart(t *testing.T) {
	at := func(day, hour, minute int) time.Time { return time.Date(2026, 10, day, hour, minute, 0, 0, prague) }
	for _, c := range []struct {
		name, kind string
		at, want   time.Time
	}{
		{"a promo body by day", ClipPromoBody, at(14, 10, 0), at(14, 10, 0)},
		{"a promo body at dawn", ClipPromoBody, at(14, 5, 40), at(14, 5, 40)},
		{"a bulletin's own clip", BulletinNews, at(14, 10, 0), at(14, 10, 0)},
		{"an ident by day waits for the night", ClipIdentFull, at(14, 10, 0), at(15, 1, 0)},
		{"an ident late in the evening", ClipIdentFull, at(14, 23, 50), at(15, 1, 0)},
		{"an ident just after midnight waits for one", ClipIdentFull, at(15, 0, 20), at(15, 1, 0)},
		{"an ident at one", ClipIdentFull, at(15, 1, 0), at(15, 1, 0)},
		{"an ident that is over by half past five", ClipIdentFull, at(15, 3, 30), at(15, 3, 30)},
		{"an ident that would not be", ClipIdentFull, at(15, 3, 31), at(16, 1, 0)},
		{"a trailer by day", ClipTrailerBody, at(14, 15, 0), at(15, 1, 0)},
		{"a trailer in the small hours", ClipTrailerBody, at(15, 2, 0), at(15, 2, 0)},
		{"a trailer that is over by half past five", ClipTrailerBody, at(15, 5, 5), at(15, 5, 5)},
		{"a trailer that would not be", ClipTrailerBody, at(15, 5, 6), at(16, 1, 0)},
	} {
		if got := longRenderStart(c.at, c.kind, prague); !got.Equal(c.want) {
			t.Errorf("%s: %s, want %s", c.name, got.In(prague), c.want)
		}
	}
	// the day's change of clocks does not move the night
	loc := prague
	spring := time.Date(2027, 3, 28, 10, 0, 0, 0, loc)
	if got, want := longRenderStart(spring, ClipIdentFull, loc), time.Date(2027, 3, 29, 1, 0, 0, 0, loc); !got.Equal(want) {
		t.Errorf("across the change of clocks: %s, want %s", got, want)
	}

	// with the bulletins off nothing waits for them
	cfg := CouchTvConfig{TimeZone: "Europe/Prague"}
	r := &composition{Composer: &Composer{}, cfg: cfg, now: at(14, 10, 0)}
	airs := at(14, 11, 0)
	if got, want := r.renderTime(ClipIdentFull, airs), renderAt(r.now, airs, prague); !got.Equal(want) {
		t.Errorf("bulletins off: %s, want %s", got, want)
	}
	r.cfg.Bulletins.Enabled = true
	if got, want := r.renderTime(ClipIdentFull, airs), at(15, 1, 0); !got.Equal(want) {
		t.Errorf("bulletins on, an ident for a break in an hour: %s, want %s", got.In(prague), want)
	}
	if got, want := r.renderTime(ClipSqueeze, airs), r.now; !got.Equal(want) {
		t.Errorf("bulletins on, a squeeze for a break in an hour: %s, want %s", got.In(prague), want)
	}
}
