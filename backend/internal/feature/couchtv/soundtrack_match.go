package couchtv

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// The soundtrack finder matches a title to its own music in the catalogs: the album
// that is the title's soundtrack (or score), and on it the track a promo suits best (the
// main title, the theme, the title track). Catalog searches are noisy: "iron man
// soundtrack" brings Black Sabbath, an AC/DC album called "Iron Man 2" and "Epic
// Version" covers before Ramin Djawadi's score, and "simpsons theme" covers before the
// show's own recording, so every candidate is scored on what its names say, whether it
// is the original recording, its year and the marks of a cover.

// SoundtrackTitle is what the finder knows of a title.
type SoundtrackTitle struct {
	// Name is the English name; Aliases are other English names it was released under
	// (TMDB's US and British titles: "Harry Potter and the Sorcerer's Stone").
	Name    string
	Aliases []string
	Year    int // the release or first-air year; 0 when unknown
	Series  bool
	// Composers are TMDB's original music and theme composers; none when unknown.
	Composers []string
}

// SoundtrackCandidate is a catalog track scored for a title.
type SoundtrackCandidate struct {
	SoundtrackTrack
	// Confidence is how sure the finder is that the track is the title's own music and
	// suits a promo, 0 to 1.
	Confidence float64
}

// soundtrackConfident is the confidence a finder's pick needs to be used without an
// admin choosing it.
const soundtrackConfident = 0.7

// soundtrackCandidates is how many candidates a search keeps, soundtracksPerAlbum of
// any one album.
const (
	soundtrackCandidates = 12
	soundtracksPerAlbum  = 5
)

// soundtrackAlbumsListed is how many of each catalog's best albums are listed in full.
const soundtrackAlbumsListed = 2

type soundtrackQuery struct {
	albums bool // an album search, else a track search
	text   string
}

// soundtrackQueries are the searches for a title: its soundtrack albums and tracks, a
// series' theme, the composer's music of it; or the admin's own query.
func soundtrackQueries(t SoundtrackTitle, custom string) []soundtrackQuery {
	if custom = strings.TrimSpace(custom); custom != "" {
		return []soundtrackQuery{{albums: true, text: custom}, {text: custom}}
	}
	qs := []soundtrackQuery{{albums: true, text: t.Name + " soundtrack"}, {text: t.Name + " soundtrack"}}
	if t.Series {
		qs = append(qs, soundtrackQuery{text: t.Name + " theme"})
	}
	if len(t.Composers) > 0 {
		qs = append(qs, soundtrackQuery{text: t.Composers[0] + " " + t.Name})
	}
	return qs
}

// findSoundtracks searches the catalogs and ranks what they offer, best first. A catalog
// that fails is left out of the ranking and named in the error.
func findSoundtracks(ctx context.Context, catalogs []SoundtrackCatalog, t SoundtrackTitle, custom string) ([]SoundtrackCandidate, error) {
	tw := newTitleWords(t)
	var tracks []SoundtrackTrack
	var failures []error
	for _, c := range catalogs {
		found, err := searchCatalog(ctx, c, tw, soundtrackQueries(t, custom))
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", c.Name(), err))
		}
		tracks = append(tracks, found...)
	}
	return rankSoundtracks(tw, tracks), errors.Join(failures...)
}

// searchCatalog runs the queries on one catalog and lists its best albums in full.
func searchCatalog(ctx context.Context, c SoundtrackCatalog, tw *titleWords, queries []soundtrackQuery) ([]SoundtrackTrack, error) {
	var tracks []SoundtrackTrack
	var albums []SoundtrackAlbum
	for _, q := range queries {
		if q.albums {
			found, err := c.Albums(ctx, q.text)
			if err != nil {
				return tracks, err
			}
			albums = append(albums, found...)
			continue
		}
		found, err := c.Tracks(ctx, q.text)
		if err != nil {
			return tracks, err
		}
		tracks = append(tracks, found...)
	}
	// the albums the searches named most like the title, from album and track searches
	type scored struct {
		album SoundtrackAlbum
		score float64
	}
	seen := map[string]bool{}
	var best []scored
	consider := func(a SoundtrackAlbum) {
		if a.ID == "" || seen[a.ID] {
			return
		}
		seen[a.ID] = true
		if s := tw.albumScore(a); s >= 0.5 {
			best = append(best, scored{a, s})
		}
	}
	for _, a := range albums {
		consider(a)
	}
	for _, t := range tracks {
		consider(t.Album)
	}
	slices.SortStableFunc(best, func(a, b scored) int { return cmp.Compare(b.score, a.score) })
	for _, b := range best[:min(len(best), soundtrackAlbumsListed)] {
		found, err := c.AlbumTracks(ctx, b.album)
		if err != nil {
			return tracks, err
		}
		tracks = append(tracks, found...)
	}
	return tracks, nil
}

// rankSoundtracks scores the tracks, keeps each once (a track found twice keeps its
// best search place and the album as its listing names it; the same recording from two
// catalogs keeps the better scored, iTunes on a tie for its AAC) and orders them.
func rankSoundtracks(tw *titleWords, tracks []SoundtrackTrack) []SoundtrackCandidate {
	byID := map[string]int{}
	var merged []SoundtrackTrack
	for _, t := range tracks {
		key := t.Provider + "/" + t.ID
		i, ok := byID[key]
		if !ok {
			byID[key] = len(merged)
			merged = append(merged, t)
			continue
		}
		m := &merged[i]
		if t.Order >= 0 && (m.Order < 0 || t.Order < m.Order) {
			m.Order = t.Order
		}
		if t.Order < 0 {
			// an album listing knows the album's year, genre and artist
			m.Album, m.Number = t.Album, t.Number
		}
	}
	byRank := func(a, b SoundtrackCandidate) int {
		return cmp.Or(cmp.Compare(b.Confidence, a.Confidence), cmp.Compare(catalogRank(a.Provider), catalogRank(b.Provider)))
	}
	scored := make([]SoundtrackCandidate, 0, len(merged))
	for _, t := range merged {
		if c := tw.score(t); c > 0 {
			scored = append(scored, SoundtrackCandidate{SoundtrackTrack: t, Confidence: c})
		}
	}
	slices.SortStableFunc(scored, byRank)
	kept := map[string]int{}
	var keys []string
	for i, c := range scored {
		key := strings.Join(words(c.Name), " ") + "|" + strings.Join(words(c.Artist), " ")
		j, ok := kept[key]
		switch {
		case !ok:
			kept[key] = i
			keys = append(keys, key)
		case c.Provider == CatalogITunes && scored[j].Provider != CatalogITunes && c.Confidence >= scored[j].Confidence-0.03:
			kept[key] = i
		}
	}
	ranked := make([]SoundtrackCandidate, 0, len(keys))
	for _, k := range keys {
		ranked = append(ranked, scored[kept[k]])
	}
	slices.SortStableFunc(ranked, byRank)
	// a few tracks of each album, so the admin also sees the other albums
	perAlbum := map[string]int{}
	out := ranked[:0]
	for _, c := range ranked {
		album := c.Provider + "/" + strings.Join(words(c.Album.Name), " ")
		if perAlbum[album]++; perAlbum[album] <= soundtracksPerAlbum && len(out) < soundtrackCandidates {
			out = append(out, c)
		}
	}
	return out
}

func catalogRank(name string) int {
	if name == CatalogITunes {
		return 0
	}
	return 1
}

// --- names ----------------------------------------------------------------------------

var deaccentText = transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)

var romanNumbers = map[string]string{"i": "1", "ii": "2", "iii": "3", "iv": "4", "v": "5", "vi": "6", "vii": "7", "viii": "8"}

// words are a name's lowercase words without accents or punctuation, so names compare
// whatever a catalog's style: an apostrophe joins ("Hedwig's" is "hedwigs"), & is "and",
// "pt" is "part" and a roman numeral after "part" is its number.
func words(s string) []string {
	if d, _, err := transform.String(deaccentText, s); err == nil {
		s = d
	}
	var out []string
	var b strings.Builder
	flush := func() {
		if b.Len() > 0 {
			out = append(out, b.String())
			b.Reset()
		}
	}
	for _, r := range strings.ToLower(s) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		case r == '\'' || r == '’' || r == '`':
		case r == '&' || r == '+':
			flush()
			out = append(out, "and")
		default:
			flush()
		}
	}
	flush()
	for i, w := range out {
		if w == "pt" {
			out[i] = "part"
		}
		if n, ok := romanNumbers[w]; ok && i > 0 && out[i-1] == "part" {
			out[i] = n
		}
	}
	return out
}

func setOf(ws ...string) map[string]bool {
	m := make(map[string]bool, len(ws))
	for _, w := range ws {
		m[w] = true
	}
	return m
}

// stopWords never tell two names apart.
var stopWords = setOf("the", "a", "an", "and", "of", "in", "on", "at", "to", "for", "by", "with", "from")

// releaseWords say what a release is rather than what it is from ("Iron Man (Original
// Motion Picture Soundtrack)", "The Simpsons Main Title Theme"): names are compared
// without them, unless the title has them itself.
var releaseWords = setOf("original", "soundtrack", "soundtracks", "score", "scores", "music", "theme", "themes", "main",
	"title", "titles", "song", "songs", "opening", "closing", "end", "credits", "suite", "version", "ost", "album",
	"edition", "deluxe", "expanded", "special", "anniversary", "remastered", "remaster", "official", "complete", "single",
	"ep", "extended", "inspired", "collectors", "limited")

// filmWords and showWords describe a film's or a show's music, and name a different work
// for the other kind: "The Simpsons Movie: The Music" is not the series' own.
var (
	filmWords = setOf("motion", "picture", "pictures", "film", "films", "movie")
	showWords = setOf("series", "television", "tv", "show")
)

// countedWords number a release within a work ("Season 1", "Vol. 2"), unlike "Part 1",
// which is a work of its own.
var countedWords = setOf("season", "seasons", "series", "volume", "vol", "chapter", "disc", "cd", "book", "episode")

func isNumber(w string) bool {
	for _, r := range w {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return w != ""
}

func isYear(w string) bool {
	return len(w) == 4 && isNumber(w) && (w[:2] == "19" || w[:2] == "20")
}

func numbers(w []string) string {
	var ns []string
	for _, x := range w {
		if isNumber(x) {
			ns = append(ns, x)
		}
	}
	slices.Sort(ns)
	return strings.Join(ns, " ")
}

var (
	bracketed = regexp.MustCompile(`[(\[]([^)\]]*)[)\]]`)
	quoted    = regexp.MustCompile(`["“”„]([^"“”„]+)["“”„]`)
	fromRef   = regexp.MustCompile(`(?i)\bfrom\s+(.+)$`)
	singleEnd = regexp.MustCompile(`(?i)\s+-\s+(single|ep)\s*$`)
)

// base is a release's name without its brackets ("Hedwig's Theme").
func base(name string) string {
	return strings.TrimSpace(bracketed.ReplaceAllString(singleEnd.ReplaceAllString(name, ""), " "))
}

// fromNames are the works a release says it is from: "Hedwig's Theme (From "Harry Potter
// and the Philosopher's Stone")", "Music from the Simpsons".
func fromNames(name string) []string {
	refs := []string{name}
	for _, m := range bracketed.FindAllStringSubmatch(name, -1) {
		refs = append(refs, m[1])
	}
	var out []string
	for _, r := range refs {
		m := fromRef.FindStringSubmatch(r)
		if m == nil {
			continue
		}
		from := bracketed.ReplaceAllString(m[1], " ")
		if q := quoted.FindStringSubmatch(m[1]); q != nil {
			from = q[1]
		}
		if !slices.Contains(out, from) {
			out = append(out, from)
		}
	}
	return out
}

// nameSimilarity is how alike two cleaned names are, 0 to 1, by the words they share in
// order: one word in four differing is "Sorcerer's Stone" against "Philosopher's Stone".
// A different number is a different work: Iron Man is not Iron Man 2.
func nameSimilarity(title, name []string) float64 {
	if len(title) == 0 || len(name) == 0 || numbers(title) != numbers(name) {
		return 0
	}
	return 1 - float64(editDistance(title, name))/float64(max(len(title), len(name)))
}

func editDistance(a, b []string) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}

// contains reports whether part appears in w as consecutive words.
func contains(w, part []string) bool {
	if len(part) == 0 {
		return false
	}
	for i := 0; i+len(part) <= len(w); i++ {
		if slices.Equal(w[i:i+len(part)], part) {
			return true
		}
	}
	return false
}

func hasAnyPhrase(w []string, phrases ...string) bool {
	for _, p := range phrases {
		if contains(w, strings.Fields(p)) {
			return true
		}
	}
	return false
}

// --- scoring --------------------------------------------------------------------------

// titleWords is a title prepared for matching.
type titleWords struct {
	SoundtrackTitle
	names     [][]string // the name and its aliases, cleaned
	own       map[string]bool
	composers [][]string
}

func newTitleWords(t SoundtrackTitle) *titleWords {
	tw := &titleWords{SoundtrackTitle: t, own: map[string]bool{}}
	all := append([]string{t.Name}, t.Aliases...)
	for _, n := range all {
		for _, w := range words(n) {
			tw.own[w] = true
		}
	}
	for _, n := range all {
		if w := tw.clean(n); len(w) > 0 {
			tw.names = append(tw.names, w)
		}
	}
	for _, c := range t.Composers {
		if w := words(c); len(w) > 0 {
			tw.composers = append(tw.composers, w)
		}
	}
	return tw
}

// clean is a name's words that may name a work: without stop words, release words, a
// counted part or a year, unless the title has them.
func (tw *titleWords) clean(name string) []string {
	w := words(name)
	out := make([]string, 0, len(w))
	for i := 0; i < len(w); i++ {
		x := w[i]
		describes := releaseWords[x] || tw.Series && showWords[x] || !tw.Series && filmWords[x]
		switch {
		case countedWords[x] && !tw.own[x] && i+1 < len(w) && isNumber(w[i+1]):
			i++
		case stopWords[x], describes && !tw.own[x], isYear(x) && !tw.own[x]:
		default:
			out = append(out, x)
		}
	}
	return out
}

// otherWorkPhrases name a game's or a stage show's music, not a film's or a series'.
var otherWorkPhrases = []string{"video game", "videogame", "game soundtrack", "original game", "the game", "game music",
	"musical", "broadway", "cast recording", "on ice"}

// otherWork reports whether a release is a game's or a stage show's (unless the title's
// name says it is one).
func (tw *titleWords) otherWork(names ...string) bool {
	for _, n := range names {
		w := words(n)
		for _, p := range otherWorkPhrases {
			if f := strings.Fields(p); contains(w, f) && !tw.own[f[len(f)-1]] {
				return true
			}
		}
	}
	return false
}

// similarity is the best match of the title's names among a release's names.
func (tw *titleWords) similarity(names ...string) float64 {
	best := 0.0
	for _, n := range names {
		c := tw.clean(n)
		for _, t := range tw.names {
			best = max(best, nameSimilarity(t, c))
		}
	}
	return best
}

// about reports whether a name has the title in it among other words: "Chef Aid: The
// South Park Album".
func (tw *titleWords) about(name string) bool {
	c := tw.clean(name)
	for _, t := range tw.names {
		if contains(c, t) {
			return true
		}
	}
	return false
}

// titleTrack reports whether a track is named after the title, or after the part of it
// that sets it apart from its franchise ("The Chamber of Secrets").
func (tw *titleWords) titleTrack(name string, sim float64) bool {
	if sim >= 0.8 {
		return true
	}
	c := tw.clean(base(name))
	for _, t := range tw.names {
		if len(c) >= 2 && contains(t, c) && 2*len(c) >= len(t) {
			return true
		}
	}
	return false
}

// soundtrackPhrases are how albums say they are a film's or a show's music.
var soundtrackPhrases = []string{"soundtrack", "original score", "motion picture", "music from", "original music",
	"film score", "the score", "ost", "television series", "tv series", "series score"}

// The marks of a cover or a novelty: these never air as a title's own music.
var coverPhrases = []string{"karaoke", "tribute", "cover", "covers", "made famous", "in the style of", "originally performed",
	"piano version", "epic version", "8 bit", "8bit", "lullaby", "lullabies", "rock version", "metal version", "metal cover",
	"orchestral version", "string quartet", "music box", "bossa nova", "jazz version", "acapella", "a cappella", "parody",
	"instrumental version", "guitar version", "ukulele", "sped up", "slowed", "nightcore", "trailer music", "lofi", "lo fi",
	"piano solo", "solo piano", "for piano", "on piano", "piano remake", "for guitar", "cello version", "violin version",
	"symphonic version", "rendition", "performs", "plays the", "trailer soundtrack", "epic theme", "remix", "remixes",
	"remixed", "mixes", "inspired from", "music inspired"}

// coverArtists record film and TV themes again for compilations: a heuristic list of the
// usual names, matched by whole words.
var coverArtists = []string{"london music works", "city of prague philharmonic", "vitamin string quartet", "geek music",
	"the hit crew", "kids superstars", "soundtrack guru", "cinematic legacy", "lorchestra cinematique", "screen team",
	"firelight film orchestra", "movie sounds unlimited", "tv themes", "hollywood studio orchestra", "starlite orchestra",
	"global stage orchestra", "the evolved", "alala", "jartisto", "piano dreamers", "rockabye baby", "8 bit arcade",
	"twinkle twinkle little rock star", "tv sounds unlimited", "dominik hauser", "movie soundtrack all stars",
	"orlando pops orchestra", "sheet music boss", "voidoid", "the toonosaurs", "tv theme song library",
	"original movies orchestra"}

// coverFit is 1 for a release that shows no mark of a cover, else small.
func coverFit(names ...string) float64 {
	for _, n := range names {
		w := words(n)
		if hasAnyPhrase(w, coverPhrases...) || hasAnyPhrase(w, coverArtists...) {
			return 0.3
		}
	}
	return 1
}

// compilationPhrases are albums gathering many works' music: their tracks belong to the
// title only when their own names say so.
var compilationPhrases = []string{"greatest hits", "best of", "collection", "anthology", "essential", "essentials",
	"the music of", "hits", "themes", "trilogy", "saga", "film music", "movie music", "tv music", "television music",
	"blockbusters", "classics", "favorites", "favourites", "greatest", "superheroes", "heroes", "anthems", "playlist",
	"tunes", "experience", "loops", "chart"}

// yearFit compares an album's year with the title's: a film's soundtrack comes out with
// it; a series' any time after its first season.
func (tw *titleWords) yearFit(year int) float64 {
	switch {
	case year == 0 || tw.Year == 0:
		return 0.92
	case tw.Series && year >= tw.Year-1:
		return 1
	case tw.Series:
		return 0.6
	}
	switch d := year - tw.Year; {
	case d >= -1 && d <= 1:
		return 1
	case d >= -3 && d <= 3:
		return 0.85
	default:
		return 0.55
	}
}

// composer reports whether TMDB named one of the artists among the title's composers.
func (tw *titleWords) composer(artists ...string) bool {
	for _, a := range artists {
		w := words(a)
		for _, c := range tw.composers {
			if contains(w, c) {
				return true
			}
		}
	}
	return false
}

// single reports whether a track's album is a single: so named, or named as the track is.
// iTunes names its singles; a Deezer search does not say, so there an album named like
// its track without saying it is a soundtrack is one ("South Park Theme (From "South
// Park")").
func (tw *titleWords) single(t SoundtrackTrack) bool {
	if t.Album.Single || singleEnd.MatchString(t.Album.Name) || slices.Equal(words(t.Album.Name), words(t.Name)) {
		return true
	}
	return t.Provider == CatalogDeezer && t.Number == 0 && slices.Equal(tw.clean(t.Album.Name), tw.clean(t.Name)) &&
		!hasAnyPhrase(words(t.Album.Name), soundtrackPhrases...)
}

// soundtrackAlbum reports whether an album says it is a film's or a show's music: in its
// name ("Original Motion Picture Soundtrack", "Chef Aid: The South Park Album") or by its
// catalog genre.
func (tw *titleWords) soundtrackAlbum(a SoundtrackAlbum) bool {
	w := words(a.Name)
	return a.Soundtrack || hasAnyPhrase(w, soundtrackPhrases...) || contains(w, []string{"album"}) && tw.about(a.Name)
}

// albumScore is how surely an album is the title's soundtrack, 0 to 1: the album
// listings worth their requests.
func (tw *titleWords) albumScore(a SoundtrackAlbum) float64 {
	score := tw.similarity(a.Name, base(a.Name))
	if tw.about(a.Name) {
		score = max(score, 0.6)
	}
	if !tw.soundtrackAlbum(a) || a.Single || singleEnd.MatchString(a.Name) {
		score *= 0.7
	}
	if len(tw.composers) > 0 && !tw.composer(a.Artist) {
		score *= 0.85
	}
	if tw.otherWork(a.Name) {
		score *= 0.5
	}
	return score * tw.yearFit(a.Year) * coverFit(a.Name, a.Artist)
}

// score is a track's confidence: how surely it is the title's own music, from the
// original recording, times how well it suits a promo.
func (tw *titleWords) score(t SoundtrackTrack) float64 {
	trackSim := tw.similarity(t.Name, base(t.Name))
	albumSim := tw.similarity(t.Album.Name, base(t.Album.Name))
	fromSim := tw.similarity(append(fromNames(t.Name), fromNames(t.Album.Name)...)...)
	// a theme in brackets is a sequel's reprise ("Lumos! (Hedwig's Theme)"), a main title
	// anywhere is the title's own
	themed := hasAnyPhrase(words(base(t.Name)), "theme", "main title", "main titles", "opening") ||
		hasAnyPhrase(words(t.Name), "main title", "main titles", "main theme", "opening titles", "end credits")
	isSingle := tw.single(t)
	soundtrack := tw.soundtrackAlbum(t.Album)

	// what says the music is the title's: its album named after it, its own name, or what
	// it says it is from
	albumEvidence := albumSim
	if !soundtrack || isSingle {
		albumEvidence *= 0.7
	}
	trackEvidence := trackSim
	switch {
	case themed:
		trackEvidence *= 0.95
	case soundtrack:
		trackEvidence *= 0.8
	default:
		// a song that merely shares the title's name: Black Sabbath's "Iron Man"
		trackEvidence *= 0.45
	}
	evidence := max(albumEvidence, trackEvidence, 0.85*fromSim)

	// what says it is the original recording, not someone else's: the composer, the show
	// itself as the artist, an album of the title's music. A theme alone on a single or a
	// compilation is anyone's.
	official := 0.55
	if !isSingle {
		albumOfTitle := albumSim / 0.75
		if tw.about(t.Album.Name) {
			albumOfTitle = max(albumOfTitle, 1)
		}
		if !soundtrack {
			albumOfTitle *= 0.85
		}
		official = min(max(official, albumOfTitle), 1)
	}
	switch {
	case tw.composer(t.Artist, t.Album.Artist), tw.similarity(t.Artist) >= 0.9, tw.similarity(t.Album.Artist) >= 0.9:
		official = 1
	case len(tw.composers) > 0 && official < 1:
		// someone else's recording, off the title's own album
		official *= 0.85
	}

	match := evidence * official * tw.yearFit(t.Album.Year) * coverFit(t.Name, t.Album.Name, t.Artist, t.Album.Artist)
	if albumSim < 0.9 && hasAnyPhrase(words(t.Album.Name), compilationPhrases...) {
		match *= 0.65
	}
	if tw.otherWork(t.Album.Name, t.Name) {
		match *= 0.5
	}
	match = min(match, 1)
	fit := trackFit(words(t.Name), t, themed, tw.titleTrack(t.Name, trackSim))
	return math.Round(match*(0.7+0.3*fit)*1000) / 1000
}

// dialogueWords mark a soundtrack's spoken tracks.
var dialogueWords = setOf("dialogue", "dialog", "interview", "commentary", "trailer", "skit", "outtake", "spoken")

// trackFit is how well a track suits a promo, 0 to 1: a theme or the title track before
// the score's cues, never dialogue, long enough for a promo's 20 seconds.
func trackFit(name []string, t SoundtrackTrack, themed, titleTrack bool) float64 {
	if slices.ContainsFunc(name, func(w string) bool { return dialogueWords[w] }) {
		return 0.05
	}
	fit := 0.55
	switch {
	case themed && hasAnyPhrase(name, "main title", "main titles", "main theme", "title theme", "theme song",
		"opening title", "opening titles", "opening theme"):
		fit = 1
	case themed, titleTrack:
		fit = 0.9
	case hasAnyPhrase(name, "suite", "prologue", "overture", "finale", "opening", "end credits", "end title", "end titles"):
		fit = 0.75
	}
	switch t.Number {
	case 1:
		fit += 0.06
	case 2:
		fit += 0.03
	}
	if t.Order >= 0 && t.Order < 5 {
		// popular: near the top of a search
		fit += 0.08 * float64(5-t.Order) / 5
	}
	switch d := t.Duration; {
	case d == 0:
		fit *= 0.9
	case d < 40*time.Second:
		fit *= 0.5
	case d < 75*time.Second:
		fit *= 0.85
	}
	if hasAnyPhrase(name, "remix", "live", "demo", "acoustic", "edit", "mix", "version", "reprise", "instrumental",
		"remastered") {
		fit *= 0.8
	}
	return min(fit, 1)
}
