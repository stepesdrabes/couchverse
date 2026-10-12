package library

import (
	"bufio"
	"compress/gzip"
	"math"
	"math/rand/v2"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"couchverse/internal/media"
)

// frame is a sample of the credits detector's size, grey unless painted.
func frame(luma, chroma byte) media.Frame {
	f := media.Frame{Width: creditsW, Height: creditsH,
		Y: make([]byte, creditsW*creditsH), U: make([]byte, creditsW*creditsH/4), V: make([]byte, creditsW*creditsH/4)}
	for i := range f.Y {
		f.Y[i] = luma
	}
	for i := range f.U {
		f.U[i], f.V[i] = chroma, 128
	}
	return f
}

// crawl paints white lines of text on black, scrolled up by offset rows.
func crawl(offset int) media.Frame {
	f := frame(16, 128)
	for y := range creditsH {
		line := (y + offset) % 24
		if line < 6 {
			for x := 100 + (y+offset)/24*7%60; x < 380; x++ {
				if (x/5)%3 != 0 {
					f.Y[y*creditsW+x] = 230
				}
			}
		}
	}
	return f
}

// scene is a busy, colourful picture shifted by dx columns and dy rows.
func scene(r *rand.Rand, dx, dy int) media.Frame {
	f := frame(0, 128)
	base := make([]byte, (creditsW+64)*(creditsH+64))
	for i := range base {
		base[i] = byte(40 + r.IntN(180))
	}
	for y := range creditsH {
		for x := range creditsW {
			f.Y[y*creditsW+x] = base[(y+dy+32)*(creditsW+64)+x+dx+32]
		}
	}
	for i := range f.U {
		f.U[i] = 180
	}
	return f
}

func TestLooks(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	card := crawl(0)
	if !isCard(card) {
		t.Error("text on black is not a card")
	}
	if isCard(frame(16, 128)) {
		t.Error("plain black is a card")
	}
	if isCard(scene(r, 0, 0)) {
		t.Error("a colourful scene is a card")
	}
	prev, cur := crawl(0), crawl(7)
	if got := scrollRows(prev, cur); got != 7 {
		t.Errorf("a crawl of 7 rows reads %d", got)
	}
	if got := scrollRows(prev, prev); got != 0 {
		t.Errorf("a still card scrolls %d", got)
	}
	if got := scrollRows(scene(rand.New(rand.NewPCG(5, 6)), 0, 0), scene(rand.New(rand.NewPCG(5, 6)), 9, 0)); got != 0 {
		t.Errorf("a pan scrolls %d", got)
	}
	if got := scrollRows(frame(16, 128), frame(16, 128)); got != 0 {
		t.Errorf("black scrolls %d", got)
	}
}

func TestCreditsLikeNeedsASteadyCrawl(t *testing.T) {
	looks := []look{{scroll: 5}, {scroll: 6}, {scroll: 0}, {scroll: 9}, {scroll: 2}, {card: true}}
	got := creditsLike(looks)
	want := []bool{true, true, false, false, false, true}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("sample %d: %v, want %v", i, got[i], want[i])
		}
	}
}

// samples builds a detector track: seconds of film, then seconds of credits, then of
// film again (a post-credits scene).
func samples(film, credits, after float64) []bool {
	var out []bool
	for range int(film / creditsStep) {
		out = append(out, false)
	}
	for i := range int(credits / creditsStep) {
		out = append(out, i%10 != 9) // a few samples between cards
	}
	for range int(after / creditsStep) {
		out = append(out, false)
	}
	return out
}

func TestCreditsAt(t *testing.T) {
	music := func(start, end float64) media.Stretch { return media.Stretch{Start: start, End: end, Loudness: -20} }
	for _, c := range []struct {
		name     string
		like     []bool
		music    *media.Stretch
		want     float64
		wantNone bool
	}{
		{name: "a crawl to the end", like: samples(300, 420, 0), want: 300},
		{name: "a post-credits scene after the crawl", like: samples(240, 360, 120), want: 240},
		{name: "a short dark stretch is not credits", like: samples(300, 40, 380), wantNone: true},
		{name: "score under the final scene starts earlier: the crawl wins", like: samples(300, 420, 0), music: ptr(music(200, 500)), want: 300},
		{name: "music settling inside the crawl: the crawl shows it already", like: samples(300, 420, 0), music: ptr(music(400, 720)), want: 300},
		{name: "music starting later than an unconvincing crawl", like: append(samples(60, 90, 150), samples(0, 420, 0)...), music: ptr(music(300, 720)), want: 300},
		{name: "music alone running to the end", like: samples(720, 0, 0), music: ptr(music(450, 700)), want: 450},
		{name: "music alone that stops well before the end", like: samples(720, 0, 0), music: ptr(music(90, 250)), wantNone: true},
		{name: "music alone that is short", like: samples(720, 0, 0), music: ptr(music(650, 720)), wantNone: true},
	} {
		var m media.Stretch
		if c.music != nil {
			m = *c.music
		}
		got, ok := creditsAt(c.like, m, c.music != nil, 720)
		switch {
		case c.wantNone && ok:
			t.Errorf("%s: %v, want none", c.name, got)
		case !c.wantNone && (!ok || got != c.want):
			t.Errorf("%s: %v (%v), want %v", c.name, got, ok, c.want)
		}
	}
}

func ptr[T any](v T) *T { return &v }

func readGz(t *testing.T, path string) string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	sc := bufio.NewScanner(z)
	for sc.Scan() {
		b.WriteString(sc.Text())
		b.WriteByte('\n')
	}
	return b.String()
}

// spikeTail reads a movie's last 12 minutes as the S3 spike logged them: momentary
// loudness, and signalstats of every keyframe held until the next one on the detector's
// 2 fps grid (the spike had no frames at 2 fps, so the crawl test is the card test only).
func spikeTail(t *testing.T, name string) (*media.Loudness, []bool) {
	l := &media.Loudness{}
	for _, line := range strings.Split(readGz(t, "testdata/credits/"+name+".loud.gz"), "\n") {
		if f := strings.Fields(line); len(f) == 2 {
			v, _ := strconv.ParseFloat(f[1], 64)
			l.Momentary = append(l.Momentary, v)
		}
	}
	type stats struct{ t, yavg, sat, ymax float64 }
	var keyframes []stats
	frameRe := regexp.MustCompile(`^frame:\d+\s+pts:\d+\s+pts_time:([\d.]+)`)
	statRe := regexp.MustCompile(`lavfi\.signalstats\.(YAVG|SATAVG|YMAX)=([\d.]+)`)
	for _, line := range strings.Split(readGz(t, "testdata/credits/"+name+".sig.gz"), "\n") {
		if m := frameRe.FindStringSubmatch(line); m != nil {
			v, _ := strconv.ParseFloat(m[1], 64)
			keyframes = append(keyframes, stats{t: v})
		} else if m := statRe.FindStringSubmatch(line); m != nil {
			v, _ := strconv.ParseFloat(m[2], 64)
			k := &keyframes[len(keyframes)-1]
			switch m[1] {
			case "YAVG":
				k.yavg = v
			case "SATAVG":
				k.sat = v
			default:
				k.ymax = v
			}
		}
	}
	like := make([]bool, int(float64(len(l.Momentary))*media.LoudnessStep/creditsStep))
	k := 0
	for i := range like {
		for k+1 < len(keyframes) && keyframes[k+1].t-keyframes[0].t <= float64(i)*creditsStep {
			k++
		}
		s := keyframes[k]
		like[i] = s.yavg < cardMaxLuma && s.sat < cardMaxSaturation && s.ymax > brightLuma
	}
	return l, like
}

// TestCreditsOnProductionLogs decides three films of the production library from the
// S3 spike's logs of their last 12 minutes.
func TestCreditsOnProductionLogs(t *testing.T) {
	for _, c := range []struct {
		name        string
		music       [2]float64 // the credits_music range found
		credits     float64    // seconds into the tail; -1 for none
		creditsNote string
	}{
		// the crawl, under music that started over the last scene
		{"pulp-fiction", [2]float64{410, 710}, 439, "the crawl"},
		// dark designed titles from the start of the window, then colourful ones, then the
		// crawl: the music starts later than the dark titles, and the picture does not
		// confirm them all the way, so the later answer wins
		{"iron-man-2", [2]float64{220, 410}, 220, "the music"},
		// parchment credits never look dark, and the score stops well before the end
		{"harry-potter-3", [2]float64{130, 290}, -1, "none"},
	} {
		l, like := spikeTail(t, c.name)
		music, ok := creditsMusic(l)
		if !ok || music.Start != c.music[0] || music.End != c.music[1] {
			t.Errorf("%s: music %+v (%v), want %v", c.name, music, ok, c.music)
		}
		at, ok := creditsAt(like, music, ok, float64(len(l.Momentary))*media.LoudnessStep)
		switch {
		case c.credits < 0 && ok:
			t.Errorf("%s: credits at %v, want none", c.name, at)
		case c.credits >= 0 && (!ok || math.Abs(at-c.credits) > 0.5):
			t.Errorf("%s: credits at %v (%v), want %v (%s)", c.name, at, ok, c.credits, c.creditsNote)
		}
	}
}
