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
