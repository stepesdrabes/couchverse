package couchtv

import (
	"fmt"
	"math"
	"testing"
	"time"

	"couchverse/internal/media"
)

// A program with a credits marker airs up to its credits; the rest plays in the
// break's squeeze.
func TestAirsToTheCredits(t *testing.T) {
	l := testLibrary{names: map[string]string{}}
	l.series(1, "Credits", repeat(20, 40), 22, "Comedy")
	for i := range l.Series[0].Episodes {
		e := &l.Series[0].Episodes[i]
		e.CreditsAt = e.Duration - 40*time.Second
	}
	cfg := DefaultConfig()
	cfg.Template = CouchTvTemplate{Slots: []CouchTvSlot{
		{Key: "open", Days: dayNames, Start: "06:00", Anchor: true, Fill: FillStrip, Count: 2},
		{Key: "rest", Days: dayNames, Fill: FillReruns},
	}}
	rows := planWeek(l.Library, "2026-10-12", 1, func(in *PlanInput) { in.Config = cfg })
	checkTimeline(t, cfg, l, rows)
	for _, a := range programs(rows)[:2] {
		if a.CreditsAt == 0 || a.EndsAt.Sub(a.StartsAt) != a.CreditsAt {
			t.Errorf("%s airs %v with credits at %v", l.label(a), a.EndsAt.Sub(a.StartsAt), a.CreditsAt)
		}
	}
}

func TestStrongestIntro(t *testing.T) {
	m := func(file string, season, start, end int) marker {
		return marker{FileID: file, Season: season, Start: start * 1000, End: end * 1000}
	}
	intros := []marker{
		// season 1: two of ten episodes share an intro
		m("a", 1, 20, 49), m("b", 1, 20, 74),
		// season 2: three of four, of 29, 31 and 33 s
		m("c", 2, 10, 39), m("d", 2, 12, 43), m("e", 2, 11, 44),
		// season 3: one of one, but a single episode is no theme to trust over season 2's
		m("f", 3, 5, 15),
	}
	got, ok := strongestIntro(intros, map[int]int{1: 10, 2: 4, 3: 1})
	if !ok || got.FileID != "d" {
		t.Errorf("picked %+v, want season 2's median intro (d)", got)
	}
	// with every season as consistent, the bigger season wins
	more := append(intros[:5:5], m("g", 1, 20, 50), m("h", 1, 21, 52))
	got, _ = strongestIntro(more, map[int]int{1: 4, 2: 3})
	if got.FileID != "h" {
		t.Errorf("picked %+v, want season 1's median (four episodes)", got)
	}
	// a season too small to trust is still better than nothing
	got, _ = strongestIntro(intros[5:], map[int]int{3: 1})
	if got.FileID != "f" {
		t.Errorf("picked %+v, want season 3's", got)
	}
	if _, ok := strongestIntro(nil, nil); ok {
		t.Error("no intros gave one")
	}
}

// grey is a shotW x shotH frame of one level with a block of another at x.
func grey(level byte, x int, block byte) media.Frame {
	f := media.Frame{Width: shotW, Height: shotH, Y: make([]byte, shotW*shotH)}
	for i := range f.Y {
		f.Y[i] = level
		if col := i % shotW; col >= x && col < x+40 {
			f.Y[i] = block
		}
	}
	return f
}

func TestStills(t *testing.T) {
	if s := readStill(grey(16, 0, 18)); s.usable() {
		t.Errorf("near-black is usable: %+v", s)
	}
	if s := readStill(grey(120, 0, 120)); s.usable() {
		t.Errorf("a flat frame is usable: %+v", s)
	}
	if s := readStill(grey(60, 50, 220)); !s.usable() {
		t.Errorf("a contrasty frame is not usable: %+v", s)
	}
	stills := []still{
		{t: 1, mean: 100, p95: 200, contrast: 30},
		{t: 2, mean: 100, p95: 200, contrast: 50},
		{t: 12, mean: 10, p95: 20, contrast: 50}, // black
		{t: 25, mean: 100, p95: 200, contrast: 20},
		{t: 40, mean: 100, p95: 200, contrast: 90}, // outside the window
	}
	got := pickStills(stills, 0, 30, 3)
	if len(got) != 2 || got[0].t != 2 || got[1].t != 25 {
		t.Errorf("picked %+v, want the best of each usable stretch", got)
	}
}

func TestMeasureShot(t *testing.T) {
	var frames []media.Frame
	for i := range 35 {
		frames = append(frames, grey(60, i*2, 220)) // a block sliding right
	}
	length, motion := measureShot(frames)
	if length != shotMaxLength || motion <= 0 {
		t.Errorf("a moving shot: %v s, motion %v", length, motion)
	}
	// a cut after 1.5 s
	cut := append(append([]media.Frame{}, frames[:15]...), grey(200, 0, 30), grey(200, 2, 30))
	if length, _ := measureShot(cut); math.Abs(length-1.4) > 1e-9 {
		t.Errorf("cut after 1.5 s: %v s", length)
	}
	// a fade to black after a second
	fade := append(append([]media.Frame{}, frames[:10]...), grey(10, 0, 12))
	if length, _ := measureShot(fade); math.Abs(length-0.9) > 1e-9 {
		t.Errorf("a fade to black: %v s", length)
	}
}

func TestChooseShots(t *testing.T) {
	var cands []candidate
	for file := range 3 {
		for k := range 4 {
			cands = append(cands, candidate{
				shot: CouchTvShot{MediaFileID: fmt.Sprint(file), StartMs: k * 1000, Score: float64(k+file) / 10},
				file: file, t: float64(k),
			})
		}
	}
	got := chooseShots(cands, 2)
	var order []string
	for _, s := range got {
		order = append(order, fmt.Sprintf("%s@%d", s.MediaFileID, s.StartMs))
	}
	if fmt.Sprint(order) != "[0@2000 0@3000 1@2000 1@3000 2@2000 2@3000]" {
		t.Errorf("chose %v", order)
	}
	if got := chooseShots(nil, 2); got == nil || len(got) != 0 {
		t.Errorf("no candidates chose %v", got)
	}
}

func TestSpread(t *testing.T) {
	var files []titleFile
	for i := range 10 {
		files = append(files, titleFile{ID: fmt.Sprint(i)})
	}
	var ids []string
	for _, f := range spread(files, 3) {
		ids = append(ids, f.ID)
	}
	if fmt.Sprint(ids) != "[1 5 8]" {
		t.Errorf("spread %v", ids)
	}
	if len(spread(files[:2], 3)) != 2 {
		t.Error("spread made up files")
	}
}

func TestWaveform(t *testing.T) {
	momentary := []float64{-120.7, -30, -20, -25, 3, -40, -50, -60, -65}
	w := waveform("f", 1000, 1900, 300, momentary)
	if fmt.Sprint(w.Loudness) != "[-20 0 -50]" || w.StepMs != 300 {
		t.Errorf("waveform %+v", w)
	}
}
