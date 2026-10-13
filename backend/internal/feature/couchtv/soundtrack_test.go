package couchtv

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestSoundtrackFinder replays the catalogs' answers for the production library: each
// title's own music on top (each Harry Potter film from its own score album, a series'
// original recording before the covers), confident unless only covers were found.
func TestSoundtrackFinder(t *testing.T) {
	type want struct {
		track, artist, album string // substrings of the top pick's; "" for any
		confident            bool
	}
	wants := map[string]want{
		"iron-man":       {"Iron Man", "Ramin Djawadi", "Iron Man (Original Motion Picture Soundtrack)", true},
		"iron-man-2":     {"", "John Cardon Debney", "Iron Man 2 (Original Motion Picture Score)", true},
		"iron-man-3":     {"Iron Man 3", "Brian Tyler", "Iron Man 3 (Original Motion Picture Soundtrack)", true},
		"pulp-fiction":   {"", "", "Pulp Fiction (Original Motion Picture Soundtrack)", true},
		"harry-potter-1": {"Hedwig's Theme", "John Williams", "Sorcerer's Stone", true},
		"harry-potter-2": {"The Chamber of Secrets", "John Williams", "Chamber of Secrets", true},
		"harry-potter-3": {"", "John Williams", "Prisoner of Azkaban", true},
		"harry-potter-4": {"The Goblet of Fire", "Patrick Doyle", "Goblet of Fire", true},
		"harry-potter-5": {"", "Nicholas Hooper", "Order of the Phoenix", true},
		"harry-potter-6": {"Opening", "Nicholas Hooper", "Half-Blood Prince", true},
		"harry-potter-7": {"", "Alexandre Desplat", "Deathly Hallows, Pt. 1", true},
		"harry-potter-8": {"Lily's Theme", "Alexandre Desplat", "Deathly Hallows, Pt. 2", true},
		"the-simpsons":   {"Main Title Theme", "The Simpsons", "Original Music from the Television Series", true},
		"south-park":     {"South Park Theme", "Primus", "Chef Aid", true},
		"futurama":       {"Futurama Main Theme", "Christopher Tyng", "", true},
		"dexter":         {"Dexter Main Title", "Rolfe Kent", "Dexter Season 1", true},
		"invincible":     {"Invincible", "John Paesano", "Invincible: Vol. 1", true},
		"shrek":          {"", "", "Shrek (Original Motion Picture", true},
	}
	for _, f := range fixtureTitles {
		t.Run(f.file, func(t *testing.T) {
			got := findFixture(t, f.file, f.title)
			w := wants[f.file]
			if len(got) == 0 {
				t.Fatal("no candidates")
			}
			top := got[0]
			if !strings.Contains(top.Name, w.track) || !strings.Contains(top.Artist, w.artist) || !strings.Contains(top.Album.Name, w.album) {
				t.Errorf("top pick %q by %q on %q (%.3f)", top.Name, top.Artist, top.Album.Name, top.Confidence)
			}
			if confident := top.Confidence >= soundtrackConfident; confident != w.confident {
				t.Errorf("top pick %q at %.3f: confident %v, want %v", top.Name, top.Confidence, confident, w.confident)
			}
			if !slices.IsSortedFunc(got, func(a, b SoundtrackCandidate) int { return int((b.Confidence - a.Confidence) * 1e6) }) {
				t.Error("candidates are not ranked")
			}
			if len(got) > soundtrackCandidates {
				t.Errorf("%d candidates kept", len(got))
			}
		})
	}
}

// TestSoundtrackFinderRejects checks what the catalogs bring that is not the title's own
// music: never confident, whatever its name says.
func TestSoundtrackFinderRejects(t *testing.T) {
	cases := []struct {
		file   string
		artist string // a candidate by them must stay under the threshold
	}{
		{"iron-man", "Black Sabbath"},
		{"iron-man", "AC/DC"},
		{"iron-man", "Alala"},
		{"iron-man", "London Music Works"},
		{"iron-man-2", "AC/DC"},
		{"the-simpsons", "Green Day"},
		{"the-simpsons", "TV Mania Orchestra"},
		{"the-simpsons", "Hans Zimmer"},
		{"south-park", "samuraiguitarist"},
		{"south-park", "Darren Mitchell"},
		{"futurama", "The Greatest Bits"},
		{"dexter", "The Darkly Dreaming Serial Killers"},
		{"invincible", "Brunon Lubas"},
		{"harry-potter-1", "The City of Prague Philharmonic Orchestra"},
	}
	titles := map[string]SoundtrackTitle{}
	for _, f := range fixtureTitles {
		titles[f.file] = f.title
	}
	found := map[string][]SoundtrackCandidate{}
	for _, c := range cases {
		got, ok := found[c.file]
		if !ok {
			// every candidate, not only the kept ones
			catalogs, _ := fixtureCatalogs(t, c.file)
			tw := newTitleWords(titles[c.file])
			var tracks []SoundtrackTrack
			for _, cat := range catalogs {
				ts, err := searchCatalog(context.Background(), cat, tw, soundtrackQueries(titles[c.file], ""))
				if err != nil {
					t.Fatal(err)
				}
				tracks = append(tracks, ts...)
			}
			for _, tr := range tracks {
				got = append(got, SoundtrackCandidate{SoundtrackTrack: tr, Confidence: tw.score(tr)})
			}
			found[c.file] = got
		}
		for _, cand := range got {
			if cand.Artist == c.artist && cand.Confidence >= soundtrackConfident {
				t.Errorf("%s: %q by %s on %q is confident at %.3f", c.file, cand.Name, cand.Artist, cand.Album.Name, cand.Confidence)
			}
		}
	}
}

// TestSoundtrackScore covers the rules the fixtures lean on, on made-up tracks.
func TestSoundtrackScore(t *testing.T) {
	ost := SoundtrackAlbum{ID: "1", Name: "Glass Harbor (Original Motion Picture Soundtrack)", Artist: "Ada Lumen", Year: 2025,
		Soundtrack: true}
	track := func(name, artist string, album SoundtrackAlbum) SoundtrackTrack {
		return SoundtrackTrack{Provider: CatalogITunes, ID: name, Name: name, Artist: artist, Album: album, Number: 3,
			Duration: 3 * time.Minute, Order: -1}
	}
	movie := SoundtrackTitle{Name: "Glass Harbor", Year: 2025}
	tw := newTitleWords(movie)
	score := func(tw *titleWords, tr SoundtrackTrack) float64 { return tw.score(tr) }

	mainTitle := score(tw, track("Main Title", "Ada Lumen", ost))
	cue := score(tw, track("The Lighthouse", "Ada Lumen", ost))
	dialogue := score(tw, track("The Door (Dialogue)", "Ada Lumen", ost))
	if !(mainTitle > cue && cue > dialogue && cue >= soundtrackConfident) {
		t.Errorf("on the score album: main title %.3f, cue %.3f, dialogue %.3f", mainTitle, cue, dialogue)
	}

	// another film of the franchise is another album
	sequel := track("Main Title", "Ada Lumen", SoundtrackAlbum{Name: "Glass Harbor 2 (Original Motion Picture Soundtrack)",
		Year: 2027, Soundtrack: true})
	if s := score(tw, sequel); s >= soundtrackConfident {
		t.Errorf("the sequel's main title scores %.3f", s)
	}
	// a song that merely shares the name
	if s := score(tw, track("Glass Harbor", "The Wave Band", SoundtrackAlbum{Name: "Tides", Year: 1998})); s >= 0.5 {
		t.Errorf("an unrelated song named like the title scores %.3f", s)
	}
	// covers and singles by someone else
	for _, tr := range []SoundtrackTrack{
		track("Glass Harbor Theme (Piano Version)", "Keys", SoundtrackAlbum{Name: "Movie Themes on Piano", Year: 2026}),
		track("Glass Harbor Main Title", "Somebody", SoundtrackAlbum{Name: "Glass Harbor Main Title - Single", Year: 2026,
			Soundtrack: true}),
		track("Main Title (From \"Glass Harbor\")", "Big Screen Hits", SoundtrackAlbum{Name: "Greatest Movie Themes", Year: 2026}),
	} {
		if s := score(tw, tr); s >= soundtrackConfident {
			t.Errorf("%q by %s on %q scores %.3f", tr.Name, tr.Artist, tr.Album.Name, s)
		}
	}

	// TMDB's composer makes their single the original, and someone else's less likely
	composed := newTitleWords(SoundtrackTitle{Name: "Glass Harbor", Year: 2025, Composers: []string{"Ada Lumen"}})
	single := track("Glass Harbor Main Title", "Ada Lumen", SoundtrackAlbum{Name: "Glass Harbor Main Title - Single", Year: 2025})
	if s := score(composed, single); s < soundtrackConfident {
		t.Errorf("the composer's single scores %.3f", s)
	}
	if score(composed, single) <= score(tw, single) {
		t.Error("knowing the composer does not help their single")
	}
	// songs on the title's own soundtrack stay its own, whoever composed the score
	song := track("Harbor Lights", "The Wave Band", ost)
	if s := score(composed, song); s < soundtrackConfident {
		t.Errorf("a song on the film's soundtrack scores %.3f with the composer known", s)
	}

	// a series' theme by the show itself, its movie's music, its game's
	show := newTitleWords(SoundtrackTitle{Name: "The Glassworks", Year: 2010, Series: true})
	theme := SoundtrackTrack{Provider: CatalogDeezer, ID: "9", Name: "The Glassworks Main Title Theme", Artist: "The Glassworks",
		Album: SoundtrackAlbum{Name: "Songs from the Glassworks (Original Music from the Television Series)", Year: 2014}, Order: 0}
	if s := score(show, theme); s < 0.9 {
		t.Errorf("the show's own theme scores %.3f", s)
	}
	for _, album := range []string{"The Glassworks Movie: The Music", "The Glassworks: The Video Game (Original Soundtrack)"} {
		tr := track("Finale", "Someone", SoundtrackAlbum{Name: album, Year: 2015, Soundtrack: true})
		if s := score(show, tr); s >= soundtrackConfident {
			t.Errorf("%q scores %.3f for the series", album, s)
		}
	}
}

func TestSoundtrackNames(t *testing.T) {
	if got := strings.Join(words("Harry Potter & the Philosopher’s Stone, Pt. II"), " "); got != "harry potter and the philosophers stone part 2" {
		t.Errorf("words: %q", got)
	}
	if got := strings.Join(words("Amélie"), " "); got != "amelie" {
		t.Errorf("words: %q", got)
	}
	tw := newTitleWords(SoundtrackTitle{Name: "Harry Potter and the Philosopher's Stone", Aliases: []string{"Harry Potter and the Sorcerer's Stone"}})
	if s := tw.similarity("Harry Potter and the Sorcerer's Stone (Original Motion Picture Soundtrack)"); s != 1 {
		t.Errorf("an alias matches at %.2f", s)
	}
	plain := newTitleWords(SoundtrackTitle{Name: "Harry Potter and the Philosopher's Stone"})
	if s := plain.similarity("Harry Potter and the Sorcerer's Stone"); s != 0.75 {
		t.Errorf("one word differing matches at %.2f", s)
	}
	if s := plain.similarity("Harry Potter and the Chamber of Secrets"); s >= 0.6 {
		t.Errorf("another film matches at %.2f", s)
	}
	iron := newTitleWords(SoundtrackTitle{Name: "Iron Man"})
	for name, want := range map[string]float64{
		"Iron Man (Original Motion Picture Soundtrack)": 1,
		"Iron Man 2":                     0,
		"The Man with the Iron Fists":    0,
		"Iron Man Main Title Theme":      1,
		"Iron Man - Single":              1,
		"Iron Man: Vol. 1":               1,
		"Iron Man (Music from the Film)": 1,
	} {
		if s := iron.similarity(name, base(name)); s != want && !(want == 0 && s < 0.5) {
			t.Errorf("%q matches at %.2f, want %.2f", name, s, want)
		}
	}
	if got := fromNames(`Hedwig's Theme (From "Harry Potter and the Philosopher's Stone")`); len(got) != 1 ||
		got[0] != "Harry Potter and the Philosopher's Stone" {
		t.Errorf("from: %q", got)
	}
}

func TestSoundtrackQueries(t *testing.T) {
	movie := soundtrackQueries(SoundtrackTitle{Name: "Shrek"}, "")
	series := soundtrackQueries(SoundtrackTitle{Name: "Dexter", Series: true, Composers: []string{"Daniel Licht"}}, "")
	custom := soundtrackQueries(SoundtrackTitle{Name: "Dexter"}, "  dexter rolfe kent ")
	texts := func(qs []soundtrackQuery) (out []string) {
		for _, q := range qs {
			out = append(out, q.text)
		}
		return
	}
	if got := texts(movie); !slices.Equal(got, []string{"Shrek soundtrack", "Shrek soundtrack"}) || !movie[0].albums {
		t.Errorf("movie queries %q", got)
	}
	if got := texts(series); !slices.Equal(got, []string{"Dexter soundtrack", "Dexter soundtrack", "Dexter theme", "Daniel Licht Dexter"}) {
		t.Errorf("series queries %q", got)
	}
	if got := texts(custom); !slices.Equal(got, []string{"dexter rolfe kent", "dexter rolfe kent"}) {
		t.Errorf("custom queries %q", got)
	}
}

// failingCatalog answers every search with an error.
type failingCatalog struct{ name string }

func (c failingCatalog) Name() string { return c.name }
func (failingCatalog) Tracks(context.Context, string) ([]SoundtrackTrack, error) {
	return nil, errors.New("offline")
}
func (failingCatalog) Albums(context.Context, string) ([]SoundtrackAlbum, error) {
	return nil, errors.New("offline")
}
func (failingCatalog) AlbumTracks(context.Context, SoundtrackAlbum) ([]SoundtrackTrack, error) {
	return nil, errors.New("offline")
}
func (failingCatalog) Preview(context.Context, SoundtrackTrack) (io.ReadCloser, error) {
	return nil, errors.New("offline")
}

func TestSoundtrackCatalogFailures(t *testing.T) {
	ctx := context.Background()
	shrek := SoundtrackTitle{Name: "Shrek", Year: 2001}
	if got, err := findSoundtracks(ctx, []SoundtrackCatalog{failingCatalog{"itunes"}, failingCatalog{"deezer"}}, shrek, ""); err == nil || len(got) > 0 {
		t.Errorf("no catalog answered: %v, %d candidates", err, len(got))
	}
	// one catalog down still ranks the other's, and says which failed
	catalogs, _ := fixtureCatalogs(t, "shrek")
	got, err := findSoundtracks(ctx, []SoundtrackCatalog{catalogs[0], failingCatalog{"deezer"}}, shrek, "")
	if err == nil || !strings.Contains(err.Error(), "deezer: offline") || len(got) == 0 || got[0].Provider != CatalogITunes {
		t.Errorf("with Deezer down: %v, %d candidates", err, len(got))
	}

	// Deezer fails with a 200 and an error body
	deezer := NewDeezer()
	deezer.gate = nil
	deezer.HTTP = &http.Client{Transport: roundTrip(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(
			`{"error": {"type": "Exception", "message": "Quota limit exceeded", "code": 4}}`))}, nil
	})}
	if _, err := deezer.Tracks(ctx, "shrek"); err == nil || !strings.Contains(err.Error(), "Quota") {
		t.Errorf("a Deezer error answer: %v", err)
	}
	// a preview is fetched from the catalog's own hosts only
	itunes := NewITunes()
	itunes.gate = nil
	for _, u := range []string{"http://audio-ssl.itunes.apple.com/a.m4a", "https://example.com/a.m4a", "https://apple.com.example.com/a.m4a"} {
		if _, err := itunes.Preview(ctx, SoundtrackTrack{Preview: u}); err == nil {
			t.Errorf("fetched a preview from %s", u)
		}
	}
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestPreviewExcerpt(t *testing.T) {
	// 30 s at 0.1 s steps: a second's fade in, music at -14 LUFS, two seconds' fade out
	var m []float64
	for i := range 300 {
		switch {
		case i < 10:
			m = append(m, -60+float64(i)*3)
		case i >= 280:
			m = append(m, -14-float64(i-279)*3)
		default:
			m = append(m, -14)
		}
	}
	start, length, ok := previewExcerpt(m)
	if !ok || start < 300*time.Millisecond || start > time.Second || length < 26*time.Second || start+length > 29*time.Second {
		t.Errorf("excerpt %s + %s (%v)", start, length, ok)
	}
	if _, _, ok := previewExcerpt(m[:80]); ok {
		t.Error("an 8 s preview made a bed")
	}
	if _, _, ok := previewExcerpt(make([]float64, 300)); !ok {
		t.Error("a steady preview made no bed")
	}
	silent := slices.Repeat([]float64{-90}, 300)
	if _, _, ok := previewExcerpt(silent); ok {
		t.Error("silence made a bed")
	}
}

func TestPromoBed(t *testing.T) {
	cases := []struct {
		excerpt     bool
		source      string
		soundtrack  bool
		want        string
		description string
	}{
		{true, SourceAdmin, true, BedExcerpt, "an admin's excerpt comes first"},
		{true, SourceAuto, true, BedSoundtrack, "a ready soundtrack beats the analysis"},
		{true, SourceAuto, false, BedExcerpt, "the analysis until the soundtrack is ready"},
		{false, "", true, BedSoundtrack, "the soundtrack alone"},
		{false, "", false, BedNone, "nothing"},
	}
	for _, c := range cases {
		if got := promoBed(c.excerpt, c.source, c.soundtrack); got != c.want {
			t.Errorf("%s: %s, want %s", c.description, got, c.want)
		}
	}
}

// TestSoundtrackBedExtraction cuts a bed from an iTunes preview as the render sidecar
// does: FLAC, stereo, 48 kHz, the stretch the URL names, at a path that names the track.
func TestSoundtrackBedExtraction(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not installed")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe is not installed")
	}
	data := t.TempDir()
	source := filepath.Join(data, previewFile(CatalogITunes, "278216574"))
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("ffmpeg", "-y", "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i",
		"sine=frequency=440:duration=30:sample_rate=44100", "-c:a", "aac", source).CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	m := &mediaServer{dataDir: data, ffmpeg: "ffmpeg"}
	u := SoundtrackURL(CatalogITunes, "278216574", 1500*time.Millisecond, 20*time.Second)
	if u != "/media/v1/soundtrack/itunes/278216574/1500_20000.flac" {
		t.Errorf("URL %s", u)
	}
	spec := []byte(`{"template": "promo", "bed": {"url": "` + u + `", "in": 0, "out": 20}}`)
	if err := m.prepare(context.Background(), spec); err != nil {
		t.Fatal(err)
	}
	path, ok := m.local(u)
	if !ok || path != filepath.Join(data, "couchtv", "audio", "itunes-278216574", "1500_20000.flac") {
		t.Fatalf("bed at %s (%v)", path, ok)
	}
	out, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "stream=codec_name,channels,sample_rate:format=duration",
		"-of", "default=noprint_wrappers=1", path).Output()
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if !strings.Contains(got, "codec_name=flac") || !strings.Contains(got, "channels=2") || !strings.Contains(got, "sample_rate=48000") ||
		!strings.Contains(got, "duration=20.0") {
		t.Errorf("bed: %s", got)
	}
	// a preview that is not on disk fails the render rather than playing silence
	missing := SoundtrackURL(CatalogDeezer, "1", 0, 20*time.Second)
	if err := m.prepare(context.Background(), []byte(`{"bed": {"url": "`+missing+`"}}`)); err == nil {
		t.Error("a bed was cut from a preview that is not there")
	}
}

// TestSoundtrackFinderPrint prints the finder's candidates for the production titles
// (COUCHVERSE_SOUNDTRACK_PRINT=1; with COUCHVERSE_SOUNDTRACK_LIVE=1 from the catalogs).
func TestSoundtrackFinderPrint(t *testing.T) {
	if os.Getenv("COUCHVERSE_SOUNDTRACK_PRINT") != "1" {
		t.Skip("COUCHVERSE_SOUNDTRACK_PRINT is not set")
	}
	for _, f := range fixtureTitles {
		got := findFixture(t, f.file, f.title)
		t.Logf("== %s (%d)", f.title.Name, f.title.Year)
		for i, c := range got {
			t.Logf("  %d. %.3f %s: %q by %q on %q (%d)", i+1, c.Confidence, c.Provider, c.Name, c.Artist, c.Album.Name, c.Album.Year)
		}
	}
}
