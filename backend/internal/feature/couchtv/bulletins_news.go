package couchtv

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"math"
	"math/bits"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// The news bulletin: the day's most important stories from the main feeds of Czech
// outlets. No outlet publishes what is most read in a machine-readable form (ČT24's
// trending list is the one exception, and unofficial), so a story weighs as much as the
// outlets carrying it. ČTK and Economia (Aktuálně, HN) are never read: their terms forbid
// passing their texts on, or analysing them automatically.

// Outlets, as stories name them.
const (
	outletCT24         = "ct24"
	outletIRozhlas     = "irozhlas"
	outletSeznamZpravy = "seznamzpravy"
	outletNovinky      = "novinky"
	outletIDnes        = "idnes"
	outletCNNPrima     = "cnnprima"
	outletDenik        = "denik"
)

// outlet is a news outlet the bulletin reads, in the order a story's headline is taken
// from (ČT24 first). Those that may not be shown only weigh the stories they carry.
type outlet struct {
	code, name string
	shown      bool
}

var outlets = []outlet{
	{outletCT24, "ČT24", true},
	{outletIRozhlas, "iROZHLAS", true},
	{outletSeznamZpravy, "Seznam Zprávy", true},
	{outletNovinky, "Novinky.cz", true},
	{outletIDnes, "iDNES.cz", true},
	// only weighing stories (Deník's terms allow its feeds for personal use only)
	{outletCNNPrima, "CNN Prima NEWS", false},
	{outletDenik, "Deník", false},
}

func outletIndex(code string) int {
	return slices.IndexFunc(outlets, func(o outlet) bool { return o.code == code })
}

// newsFeed is a feed the bulletin reads. Some sites answer a wrong address with their
// default feed or an empty one rather than an error, so the channel's title must carry
// mark (folded).
type newsFeed struct {
	name, outlet, url, mark string
	editorsPick             bool
}

var newsFeeds = []newsFeed{
	{"ČT24", outletCT24, "https://ct24.ceskatelevize.cz/rss", "ct24", false},
	{"ČT24 výběr redakce", outletCT24, "https://ct24.ceskatelevize.cz/rss/tema/vyber-redakce-84313", "vyber redakce", true},
	{"iROZHLAS", outletIRozhlas, "https://www.irozhlas.cz/rss/irozhlas", "irozhlas", false},
	{"Seznam Zprávy", outletSeznamZpravy, "https://www.seznamzpravy.cz/rss", "seznam zpravy", false},
	{"Novinky.cz", outletNovinky, "https://www.novinky.cz/rss", "novinky", false},
	{"iDNES.cz", outletIDnes, "https://servis.idnes.cz/rss.aspx?c=zpravodaj", "idnes", false},
	{"CNN Prima NEWS", outletCNNPrima, "https://cnn.iprima.cz/rss", "cnn prima", false},
	{"Deník", outletDenik, "https://www.denik.cz/rss/zpravy.xml", "denik", false},
}

// trendingURL is ČT24's own list of its most read articles of the last 24 hours, from its
// site's scripts: undocumented, so the bulletin ranks without it when it fails or changes.
const trendingURL = "https://ct24.ceskatelevize.cz/api/trendingArticles?pageSize=8&periodHours=24"

const (
	// newsWindow is how far back a story counts
	newsWindow = 12 * time.Hour
	// newsAhead tolerates a feed's clock running a little fast; an item further ahead is wrong
	newsAhead  = time.Hour
	minStories = 3
	maxStories = 5
	// sameStory is the average similarity of their items that makes two groups one story:
	// on the feeds of 13 Oct 2026 the outlets' articles on one story mostly scored 0.3 or
	// more, different stories sharing a few words (two Hungarian ones) 0.2 to 0.26
	sameStory = 0.25
	// a story's weight halves over newsHalfLife of age: freshness counts, but less than
	// being carried by one more outlet
	newsHalfLife = 24 * time.Hour
	// the editors' selection and trending add to the outlets a story counts
	editorsPickWeight = 1.5
	trendingWeight    = 1.0
)

// newsItem is one article a feed carries, its text cleaned.
type newsItem struct {
	outlet    string
	title     string
	perex     string
	link      string
	key       string // the article within its outlet
	published time.Time
	// editorsPick and trending are ČT24's
	editorsPick, trending bool
}

// fetchNews reads every feed and ČT24's trending list at once: the feeds' news (not
// sport, culture, lifestyle, opinion or shows), the trending articles and how each source
// answered.
func fetchNews(ctx context.Context, f *fetcher) ([]newsItem, []trendingArticle, []CouchTvNewsFeed) {
	feeds := make([]CouchTvNewsFeed, len(newsFeeds)+1)
	read := make([][]newsItem, len(newsFeeds))
	var trending []trendingArticle
	var wg sync.WaitGroup
	for i, feed := range newsFeeds {
		wg.Go(func() {
			items, err := readFeed(ctx, f, feed)
			read[i] = items
			feeds[i] = CouchTvNewsFeed{Name: feed.name, Items: len(items)}
			if err != nil {
				feeds[i].Error = err.Error()
			}
		})
	}
	wg.Go(func() {
		var err error
		trending, err = readTrending(ctx, f)
		feeds[len(newsFeeds)] = CouchTvNewsFeed{Name: "ČT24 trending", Items: len(trending)}
		if err != nil {
			feeds[len(newsFeeds)].Error = err.Error()
		}
	})
	wg.Wait()
	return slices.Concat(read...), trending, feeds
}

// mergeNews keeps the items of the last newsWindow, one per article and outlet (the
// first one's text, every one's marks), marks ČT24's trending articles and adds those its
// feeds no longer carry, unless they are not news.
func mergeNews(items []newsItem, trending []trendingArticle, now time.Time) []newsItem {
	var out []newsItem
	seen := map[string]int{}
	add := func(it newsItem, labels []string) {
		if it.published.Before(now.Add(-newsWindow)) || it.published.After(now.Add(newsAhead)) || offTopic(labels) {
			return
		}
		id := it.outlet + " " + it.key
		if i, ok := seen[id]; ok {
			out[i].editorsPick = out[i].editorsPick || it.editorsPick
			out[i].trending = out[i].trending || it.trending
			return
		}
		seen[id] = len(out)
		out = append(out, it)
	}
	for _, it := range items {
		add(it, nil)
	}
	for _, a := range trending {
		add(a.item(), a.labels())
	}
	return out
}

// readFeed fetches and parses one feed.
func readFeed(ctx context.Context, f *fetcher, feed newsFeed) ([]newsItem, error) {
	body, err := f.get(ctx, feed.url)
	if err != nil {
		return nil, err
	}
	title, raw, err := parseRSS(body)
	if err != nil {
		return nil, err
	}
	if !strings.Contains(fold(title), feed.mark) {
		return nil, fmt.Errorf("the feed is %q, not %s", cleanText(title), feed.name)
	}
	if len(raw) == 0 {
		return nil, errors.New("the feed is empty")
	}
	var items []newsItem
	for _, r := range raw {
		if it, ok := r.item(feed.outlet); ok && !offTopic(r.labels()) {
			it.editorsPick = feed.editorsPick
			items = append(items, it)
		}
	}
	return items, nil
}

// rawItem is an RSS item as the feed has it.
type rawItem struct {
	title, link, description, pubDate, guid string
	// categories (their names and domains) and Seznam's sections
	categories []string
}

// item cleans a raw item; it is not one without a title, a web address or a date.
func (r rawItem) item(outlet string) (newsItem, bool) {
	link := normalizeLink(r.link)
	published, ok := parseFeedDate(r.pubDate)
	it := newsItem{outlet: outlet, title: cleanText(r.title), perex: cleanText(r.description), link: link, published: published}
	if !ok || link == "" || it.title == "" {
		return it, false
	}
	switch {
	case outlet == outletCT24 && ct24ID(link) != "":
		// ČT24's trending list knows its articles by number
		it.key = ct24ID(link)
	case strings.TrimSpace(r.guid) != "":
		it.key = strings.TrimSpace(r.guid)
	default:
		// iROZHLAS has no guid
		it.key = link
	}
	return it, true
}

// labels are what says an item's topic: its categories, its sections and the sections
// its address names (iROZHLAS has nothing else).
func (r rawItem) labels() []string {
	labels := slices.Clone(r.categories)
	if u, err := url.Parse(strings.TrimSpace(r.link)); err == nil {
		segments := strings.Split(strings.Trim(u.Path, "/"), "/")
		labels = append(labels, segments[:len(segments)-1]...)
	}
	return labels
}

var ct24Article = regexp.MustCompile(`-(\d+)/?$`)

// ct24ID is the number a ČT24 article's address ends with.
func ct24ID(link string) string {
	u, err := url.Parse(link)
	if err != nil {
		return ""
	}
	if m := ct24Article.FindStringSubmatch(u.Path); m != nil {
		return m[1]
	}
	return ""
}

const seznamNS = "https://www.seznam.cz"

// parseRSS reads an RSS 2.0 feed's channel title and items. Only RSS's own elements count:
// iDNES's media:description or Atom's link are not the item's description or link.
// HTML entities outside CDATA are read as HTML has them.
func parseRSS(body []byte) (string, []rawItem, error) {
	d := xml.NewDecoder(bytes.NewReader(body))
	d.CharsetReader = charsetReader
	d.Strict = false
	d.Entity = xml.HTMLEntity
	var title string
	var items []rawItem
	depth := 0
	for {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			if depth != 0 || title == "" && items == nil {
				return "", nil, errors.New("not an RSS feed")
			}
			return title, items, nil
		}
		if err != nil {
			return "", nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			// rss > channel > its title and items
			if depth++; depth != 3 || t.Name.Space != "" {
				continue
			}
			switch t.Name.Local {
			case "title":
				if err := d.DecodeElement(&title, &t); err != nil {
					return "", nil, err
				}
				depth--
			case "item":
				it, err := parseItem(d)
				if err != nil {
					return "", nil, err
				}
				items = append(items, it)
				depth--
			}
		case xml.EndElement:
			depth--
		}
	}
}

// parseItem reads an item's elements up to its end.
func parseItem(d *xml.Decoder) (rawItem, error) {
	var it rawItem
	for {
		tok, err := d.Token()
		if err != nil {
			return it, err
		}
		switch t := tok.(type) {
		case xml.EndElement:
			return it, nil
		case xml.StartElement:
			var field *string
			switch {
			case t.Name.Space == seznamNS && t.Name.Local == "sections":
				var s struct {
					Values []string `xml:"value"`
				}
				if err := d.DecodeElement(&s, &t); err != nil {
					return it, err
				}
				it.categories = append(it.categories, s.Values...)
				continue
			case t.Name.Space != "":
			case t.Name.Local == "title":
				field = &it.title
			case t.Name.Local == "link":
				field = &it.link
			case t.Name.Local == "description":
				field = &it.description
			case t.Name.Local == "pubDate":
				field = &it.pubDate
			case t.Name.Local == "guid":
				field = &it.guid
			case t.Name.Local == "category":
				var c string
				if err := d.DecodeElement(&c, &t); err != nil {
					return it, err
				}
				it.categories = append(it.categories, c)
				for _, a := range t.Attr {
					if a.Name.Local == "domain" {
						if u, err := url.Parse(a.Value); err == nil {
							it.categories = append(it.categories, strings.Split(strings.Trim(u.Path, "/"), "/")...)
						}
					}
				}
				continue
			}
			if field == nil {
				if err := d.Skip(); err != nil {
					return it, err
				}
				continue
			}
			if err := d.DecodeElement(field, &t); err != nil {
				return it, err
			}
		}
	}
}

// charsetReader reads a feed in the charset its XML declaration names (MAFRA's and
// Economia's sites are windows-1250).
func charsetReader(label string, input io.Reader) (io.Reader, error) {
	enc, err := htmlindex.Get(label)
	if err != nil {
		return nil, err
	}
	return enc.NewDecoder().Reader(input), nil
}

// trendingArticle is an article of ČT24's trending list.
type trendingArticle struct {
	ID    int64 `json:"id"`
	Title struct {
		Main string `json:"main"`
	} `json:"title"`
	Slug        string `json:"slug"`
	MainSection struct {
		Path string `json:"path"`
		Name string `json:"name"`
	} `json:"mainSection"`
	Publication struct {
		DatePublished int64 `json:"datePublished"`
	} `json:"publication"`
}

func readTrending(ctx context.Context, f *fetcher) ([]trendingArticle, error) {
	body, err := f.get(ctx, trendingURL)
	if err != nil {
		return nil, err
	}
	return parseTrending(body)
}

func parseTrending(body []byte) ([]trendingArticle, error) {
	var doc struct {
		Data struct {
			Articles []trendingArticle `json:"articles"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	var out []trendingArticle
	for _, a := range doc.Data.Articles {
		if a.ID > 0 && strings.TrimSpace(a.Title.Main) != "" && a.Publication.DatePublished > 0 && a.MainSection.Path != "" {
			out = append(out, a)
		}
	}
	if len(out) == 0 {
		return nil, errors.New("the trending list has no articles")
	}
	return out, nil
}

// item is the article as ČT24's feeds would have it, for one they no longer carry.
func (a trendingArticle) item() newsItem {
	id := strconv.FormatInt(a.ID, 10)
	return newsItem{outlet: outletCT24, title: cleanText(a.Title.Main), key: id, trending: true,
		link:      "https://ct24.ceskatelevize.cz/clanek/" + a.MainSection.Path + "/" + a.Slug + "-" + id,
		published: time.Unix(a.Publication.DatePublished, 0)}
}

func (a trendingArticle) labels() []string {
	return []string{a.MainSection.Path, a.MainSection.Name}
}

// Characters the text handling looks for, by number: the repo never spells an em-dash out.
const (
	noBreakSpace   = rune(0x00a0)
	zeroWidthSpace = rune(0x200b)
	byteOrderMark  = rune(0xfeff)
	enDash         = rune(0x2013)
	emDash         = rune(0x2014)
)

// cleanText makes a feed's text plain: tags dropped (descriptions carry HTML), entities
// resolved (some feeds escape them again inside CDATA), every run of spaces, no-break
// ones included, one space. An em-dash becomes the en-dash Czech typesetting uses.
func cleanText(s string) string {
	s = html.UnescapeString(stripTags(s))
	var b strings.Builder
	space := false
	for _, r := range s {
		switch {
		case unicode.IsSpace(r):
			space = true
			continue
		case r == zeroWidthSpace || r == byteOrderMark:
			continue
		case r == emDash:
			r = enDash
		}
		if space && b.Len() > 0 {
			b.WriteByte(' ')
		}
		space = false
		b.WriteRune(r)
	}
	return b.String()
}

// stripTags drops HTML tags, leaving a space for each; a "<" that starts no tag stays.
func stripTags(s string) string {
	var b strings.Builder
	for {
		i := strings.IndexByte(s, '<')
		if i < 0 || i+1 == len(s) {
			b.WriteString(s)
			return b.String()
		}
		end := strings.IndexByte(s[i:], '>')
		if end < 0 || !opensTag(s[i+1]) {
			b.WriteString(s[:i+1])
			s = s[i+1:]
			continue
		}
		b.WriteString(s[:i])
		b.WriteByte(' ')
		s = s[i+end+1:]
	}
}

// opensTag is what follows the "<" of a tag, an end tag or a comment.
func opensTag(c byte) bool {
	return c == '/' || c == '!' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// vlnka binds a one-letter preposition or conjunction (k, s, v, z, o, u, a, i) to the
// word after it with a no-break space, so a line never ends with one, as Czech
// typesetting asks.
func vlnka(s string) string {
	words := strings.Split(s, " ")
	var b strings.Builder
	for i, w := range words {
		b.WriteString(w)
		switch {
		case i == len(words)-1:
		case oneLetter(w):
			b.WriteRune(noBreakSpace)
		default:
			b.WriteByte(' ')
		}
	}
	return b.String()
}

// oneLetter is a one-letter preposition or conjunction, maybe after an opening quote or
// bracket.
func oneLetter(w string) bool {
	w = strings.TrimLeft(w, "\"'(„‚“‘«»[")
	return len(w) == 1 && strings.Contains("ksvzouaiKSVZOUAI", w)
}

// normalizeLink drops what only tracks a reader (utm_ parameters, iDNES's #utm_ fragment),
// so an article has one address; "" for anything but a web address.
func normalizeLink(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "http" && u.Scheme != "https" || u.Host == "" {
		return ""
	}
	u.Host = strings.ToLower(u.Host)
	u.Fragment, u.RawFragment = "", ""
	if u.RawQuery != "" {
		q := u.Query()
		for k := range q {
			if strings.HasPrefix(strings.ToLower(k), "utm_") {
				q.Del(k)
			}
		}
		u.RawQuery = q.Encode()
	}
	return u.String()
}

// feedDates are the date forms feeds use: GMT (ČT24, iDNES) and numeric offsets
// (iROZHLAS, Seznam), days with and without a leading zero.
var feedDates = []string{time.RFC1123Z, time.RFC1123, "Mon, 2 Jan 2006 15:04:05 -0700", "Mon, 2 Jan 2006 15:04:05 MST",
	"2 Jan 2006 15:04:05 -0700", time.RFC3339}

func parseFeedDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	for _, layout := range feedDates {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// fold is s lower-cased without diacritics.
func fold(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	out, _, err := transform.String(t, strings.ToLower(s))
	if err != nil {
		return strings.ToLower(s)
	}
	return out
}

// offTopicWords in an item's categories, sections or address say it is not news for the
// bulletin: sport, culture, lifestyle, opinion and the outlets' own shows.
var offTopicWords = map[string]bool{
	"sport": true, "fotbal": true, "hokej": true, "tenis": true,
	"kultura": true, "hudba": true, "film": true, "filmy": true, "serialy": true, "divadlo": true, "knihy": true,
	"literatura": true, "recenze": true,
	"lifestyle": true, "magazin": true, "cestovani": true, "jidlo": true, "vareni": true, "recepty": true,
	"bydleni": true, "auto": true, "showtime": true, "celebrity": true, "zena": true, "horoskopy": true,
	"komentare": true, "nazory": true, "blogy": true, "porady": true, "podcast": true, "podcasty": true,
}

// offTopicPhrases are the same in more than one word ("Životní styl", CNN Prima's show-time).
var offTopicPhrases = []string{"zivotni styl", "show time"}

func offTopic(labels []string) bool {
	for _, l := range labels {
		words := strings.FieldsFunc(fold(l), func(r rune) bool { return !unicode.IsLetter(r) })
		for _, w := range words {
			if offTopicWords[w] {
				return true
			}
		}
		joined := " " + strings.Join(words, " ") + " "
		for _, p := range offTopicPhrases {
			if strings.Contains(joined, " "+p+" ") {
				return true
			}
		}
	}
	return false
}

// stopwords carry no story: they are left out of the stems.
var stopwords = func() map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.Fields(`jsou jsme jste bude budou bylo byla byli byly jeho jeji jejich ktery ktera ktere
		kteri kterou kterym kterych podle take jako nebo kdyz proto pred mezi jeste uvedl uvedla uvedli rekl rekla rikal
		rika tento tato tyto toho tomu tech kvuli diky proti pres pote behem zatim dnes vcera zitra letos loni roku
		procent milionu miliard tisic korun mohl mohla mohou muze musi chce chteji dalsi nove novy nova nejen vsak
		pouze tedy aniz utery uterni pondeli streda ctvrtek patek sobota nedele hodin minut lide lidi jedna jeden
		jedno podle sdelil sdelila tvrdi mini oznamil oznamila`) {
		out[stemOf([]rune(w))] = true
	}
	return out
}()

func stemOf(word []rune) string {
	return string(word[:min(len(word), 5)])
}

// stems are a text's words for telling stories apart: folded, the first five letters of
// each word of four letters or more (a rough Czech stem), common words left out.
func stems(s string) []string {
	var out []string
	for _, w := range strings.FieldsFunc(fold(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		r := []rune(w)
		if len(r) < 4 || strings.Trim(w, "0123456789") == "" {
			continue
		}
		if s := stemOf(r); !stopwords[s] {
			out = append(out, s)
		}
	}
	return out
}

// similarities are the items' pairwise cosine similarities over their stems, weighed by
// how rare a stem is among the items; a stem in the title counts twice.
func similarities(items []newsItem) [][]float64 {
	n := len(items)
	vectors := make([]map[string]float64, n)
	df := map[string]int{}
	for i, it := range items {
		v := map[string]float64{}
		for _, s := range stems(it.title) {
			v[s] = 2
		}
		for _, s := range stems(it.perex) {
			if v[s] < 2 {
				v[s] = 1
			}
		}
		vectors[i] = v
		for s := range v {
			df[s]++
		}
	}
	norms := make([]float64, n)
	for i, v := range vectors {
		for s, w := range v {
			w *= math.Log(float64(n) / float64(df[s]))
			v[s] = w
			norms[i] += w * w
		}
		norms[i] = math.Sqrt(norms[i])
	}
	sim := make([][]float64, n)
	for i := range sim {
		sim[i] = make([]float64, n)
	}
	for i := range n {
		for j := i + 1; j < n; j++ {
			if norms[i] == 0 || norms[j] == 0 {
				continue
			}
			dot := 0.0
			for s, w := range vectors[i] {
				dot += w * vectors[j][s]
			}
			sim[i][j] = dot / (norms[i] * norms[j])
			sim[j][i] = sim[i][j]
		}
	}
	return sim
}

// clusterNews groups the items into stories: average-link clustering, which joins the two
// groups whose items are most alike on average while that is at least sameStory, never
// two items of one outlet (single-link chains unrelated stories together through one
// article that mentions both).
func clusterNews(items []newsItem, sim [][]float64) [][]int {
	n := len(items)
	members := make([][]int, n)
	mask := make([]uint, n)
	sum := make([][]float64, n)
	for i := range n {
		members[i] = []int{i}
		mask[i] = 1 << outletIndex(items[i].outlet)
		sum[i] = slices.Clone(sim[i])
	}
	alive := make([]bool, n)
	for i := range alive {
		alive[i] = true
	}
	for {
		bi, bj, best := -1, -1, 0.0
		for i := range n {
			if !alive[i] {
				continue
			}
			for j := i + 1; j < n; j++ {
				if !alive[j] || mask[i]&mask[j] != 0 {
					continue
				}
				if avg := sum[i][j] / float64(len(members[i])*len(members[j])); avg >= sameStory && (bi < 0 || avg > best) {
					bi, bj, best = i, j, avg
				}
			}
		}
		if bi < 0 {
			break
		}
		members[bi] = append(members[bi], members[bj]...)
		mask[bi] |= mask[bj]
		alive[bj] = false
		for k := range n {
			sum[bi][k] += sum[bj][k]
			sum[k][bi] = sum[bi][k]
		}
	}
	var out [][]int
	for i := range n {
		if alive[i] {
			out = append(out, members[i])
		}
	}
	return out
}

// rankNews tells the top stories of the items: each story weighs the outlets carrying it,
// plus editorsPickWeight when ČT24's editors chose it and trendingWeight when ČT24's
// readers did, halving over newsHalfLife since its newest article. The headline is the
// first outlet's of those that may be shown; a story none of them carries is passed over,
// and so is one that is a story already told (an outlet's second article on it).
func rankNews(items []newsItem, now time.Time) []CouchTvNewsStory {
	out := []CouchTvNewsStory{}
	if len(items) == 0 {
		return out
	}
	// one order whatever order the feeds came in, so a run ranks the same items the same way
	items = slices.Clone(items)
	slices.SortStableFunc(items, func(a, b newsItem) int {
		if c := b.published.Compare(a.published); c != 0 {
			return c
		}
		if c := strings.Compare(a.outlet, b.outlet); c != 0 {
			return c
		}
		return strings.Compare(a.key, b.key)
	})
	sim := similarities(items)
	type story struct {
		members []int
		score   float64
		newest  time.Time
	}
	var stories []story
	for _, members := range clusterNews(items, sim) {
		s := story{members: members}
		var mask uint
		pick, trending := 0.0, 0.0
		for _, m := range members {
			it := items[m]
			mask |= 1 << outletIndex(it.outlet)
			if it.editorsPick {
				pick = editorsPickWeight
			}
			if it.trending {
				trending = trendingWeight
			}
			if it.published.After(s.newest) {
				s.newest = it.published
			}
		}
		age := max(now.Sub(s.newest), 0)
		s.score = (float64(bits.OnesCount(mask)) + pick + trending) * math.Exp2(-age.Hours()/newsHalfLife.Hours())
		stories = append(stories, s)
	}
	slices.SortStableFunc(stories, func(a, b story) int {
		if a.score != b.score {
			if a.score > b.score {
				return -1
			}
			return 1
		}
		return b.newest.Compare(a.newest)
	})
	var told [][]int
	for _, s := range stories {
		head := -1
		for _, m := range s.members {
			if i := outletIndex(items[m].outlet); outlets[i].shown && (head < 0 || i < outletIndex(items[head].outlet)) {
				head = m
			}
		}
		if head < 0 || slices.ContainsFunc(told, func(t []int) bool { return averageSim(sim, t, s.members) >= sameStory }) {
			continue
		}
		told = append(told, s.members)
		it := items[head]
		story := CouchTvNewsStory{Headline: vlnka(it.title), Outlet: it.outlet, OutletName: outlets[outletIndex(it.outlet)].name,
			URL: it.link, PublishedAt: it.published, Coverage: []string{}, Score: math.Round(s.score*100) / 100}
		for _, o := range outlets {
			if slices.ContainsFunc(s.members, func(m int) bool { return items[m].outlet == o.code }) {
				story.Coverage = append(story.Coverage, o.code)
			}
		}
		for _, m := range s.members {
			story.EditorsPick = story.EditorsPick || items[m].editorsPick
			story.Trending = story.Trending || items[m].trending
		}
		out = append(out, story)
		if len(out) == maxStories {
			break
		}
	}
	return out
}

func averageSim(sim [][]float64, a, b []int) float64 {
	total := 0.0
	for _, i := range a {
		for _, j := range b {
			total += sim[i][j]
		}
	}
	return total / float64(len(a)*len(b))
}
