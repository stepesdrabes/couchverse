package couchtv

import (
	"context"
	"io"
	"math"
	"math/rand/v2"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"golang.org/x/text/encoding/charmap"
)

// typeset writes the no-break space a headline carries after a one-letter word as "~".
var typeset = strings.NewReplacer("~", string(noBreakSpace))

func feedNamed(t *testing.T, name string) newsFeed {
	t.Helper()
	i := slices.IndexFunc(newsFeeds, func(f newsFeed) bool { return f.name == name })
	if i < 0 {
		t.Fatalf("no feed %q", name)
	}
	return newsFeeds[i]
}

// Every feed as it was on 13 Oct 2026 at 17:02, sport, culture, lifestyle, opinion and
// shows left out.
func TestReadFeeds(t *testing.T) {
	f := fixtureFetcher()
	ctx := context.Background()
	counts := map[string]int{"ČT24": 9, "ČT24 výběr redakce": 11, "iROZHLAS": 3, "Seznam Zprávy": 7, "Novinky.cz": 5,
		"iDNES.cz": 5, "CNN Prima NEWS": 4, "Deník": 6}
	read := map[string][]newsItem{}
	for _, feed := range newsFeeds {
		items, err := readFeed(ctx, f, feed)
		if err != nil {
			t.Fatalf("%s: %v", feed.name, err)
		}
		if len(items) != counts[feed.name] {
			t.Errorf("%s: %d items, want %d", feed.name, len(items), counts[feed.name])
		}
		for _, it := range items {
			if it.editorsPick != feed.editorsPick || it.outlet != feed.outlet {
				t.Errorf("%s: %+v", feed.name, it)
			}
		}
		read[feed.name] = items
	}
	find := func(feed, title string) newsItem {
		t.Helper()
		for _, it := range read[feed] {
			if strings.HasPrefix(it.title, title) {
				return it
			}
		}
		t.Fatalf("%s has no %q", feed, title)
		return newsItem{}
	}

	// ČT24: GMT dates, CDATA, an article known by its number
	ct := find("ČT24", "Spojenci nás nevarovali")
	if !ct.published.Equal(time.Date(2026, 10, 13, 14, 47, 34, 0, time.UTC)) || ct.key != "378562" {
		t.Errorf("ČT24: %s %q", ct.published, ct.key)
	}
	// iROZHLAS: no guid, a +0200 date
	ir := find("iROZHLAS", "Letadlo do Norimberku")
	if ir.key != ir.link || !ir.published.Equal(time.Date(2026, 10, 13, 13, 26, 0, 0, time.UTC)) {
		t.Errorf("iROZHLAS: %q %s", ir.key, ir.published)
	}
	// Seznam: no-break spaces in the text become plain ones
	if sz := find("Seznam Zprávy", "V moři"); sz.title != "V moři u Bulharska drony opět útočily na lodě. Jednu potopily" {
		t.Errorf("Seznam: %q", sz.title)
	}
	// Novinky: the feed is not in date order, which nothing assumes
	if nv := find("Novinky.cz", "Hromadná nehoda"); !nv.published.Equal(time.Date(2026, 10, 13, 10, 41, 18, 0, time.UTC)) {
		t.Errorf("Novinky: %s", nv.published)
	}
	// iDNES: the #utm_ fragment goes, and media:description is not the item's description
	id := find("iDNES.cz", "ČEZ se bude")
	if id.link != "https://www.idnes.cz/ekonomika/podniky/cez-diag-human-soud-babis.A261013_121545_ekoakcie_iluj" ||
		!strings.HasPrefix(id.perex, "Polostátní energetická společnost ČEZ") {
		t.Errorf("iDNES: %q %q", id.link, id.perex)
	}
	// Deník: the utm_ parameters go
	if dn := find("Deník", "Bulharsko hlásí"); dn.link != "https://www.denik.cz/evropa/bulharsko-hlasi-dalsi-dronovy-utok-na-lode-v-cernem-mori-jedna-se-potopila/" {
		t.Errorf("Deník: %q", dn.link)
	}
	// the editors' selection reaches back two days
	if old := find("ČT24 výběr redakce", ""); old.title == "" {
		t.Error("the selection is empty")
	}
}

func TestParseRSSPitfalls(t *testing.T) {
	feed := `<?xml version="1.0" encoding="utf-8"?>
<rss version="2.0" xmlns:atom="http://www.w3.org/2005/Atom" xmlns:media="http://search.yahoo.com/mrss/">
<channel><atom:link href="https://example.cz/rss" rel="self"/><title>Example</title>
<image><title>Not the channel</title></image>
<item>
<title><![CDATA[Osm hodin spánku? &bdquo;Dogma&ldquo; z&nbsp;hlavy ` + string(emDash) + ` věda]]></title>
<link>https://Example.cz/a?utm_source=rss&amp;id=7#utm_medium=feed</link>
<description><![CDATA[<p>V roce 1817 &#8222;osm hodin práce&#8220;</p><img src="x.jpg"/>konec]]></description>
<pubDate>Tue, 6 Oct 2026 15:00:00 +0200</pubDate>
<media:content url="https://example.cz/x.jpg"><media:title type="html"><![CDATA[věda, spánek]]></media:title></media:content>
<media:description>A caption</media:description>
</item>
<item><title>No date</title><link>https://example.cz/b</link></item>
<item><title>A relative link</title><link>/c</link><pubDate>Tue, 13 Oct 2026 15:00:00 GMT</pubDate></item>
<item><title>An entity outside CDATA: a&nbsp;b &amp; c &lt; d</title><link>https://example.cz/d</link>
<pubDate>Tue, 13 Oct 2026 15:00:00 GMT</pubDate><guid isPermaLink="false">d-1</guid></item>
</channel></rss>`
	title, raw, err := parseRSS([]byte(feed))
	if err != nil || title != "Example" || len(raw) != 4 {
		t.Fatalf("%q, %d items, %v", title, len(raw), err)
	}
	it, ok := raw[0].item(outletIDnes)
	if !ok {
		t.Fatal("the first item is not one")
	}
	if want := "Osm hodin spánku? „Dogma“ z hlavy " + string(enDash) + " věda"; it.title != want {
		t.Errorf("title %q, want %q", it.title, want)
	}
	if it.perex != "V roce 1817 „osm hodin práce“ konec" || it.link != "https://example.cz/a?id=7" ||
		!it.published.Equal(time.Date(2026, 10, 6, 13, 0, 0, 0, time.UTC)) || it.key != it.link {
		t.Errorf("item %+v", it)
	}
	if _, ok := raw[1].item(outletIDnes); ok {
		t.Error("an item without a date counts")
	}
	if _, ok := raw[2].item(outletIDnes); ok {
		t.Error("an item without a web address counts")
	}
	if it, _ := raw[3].item(outletIDnes); it.title != "An entity outside CDATA: a b & c < d" || it.key != "d-1" {
		t.Errorf("item %+v", it)
	}
}

func TestParseRSSCharset(t *testing.T) {
	feed := `<?xml version="1.0" encoding="windows-1250"?><rss version="2.0"><channel><title>iDNES.cz</title>` +
		`<item><title>Příliš žluťoučký kůň úpěl ďábelské ódy</title><link>https://www.idnes.cz/a</link>` +
		`<pubDate>Tue, 13 Oct 2026 15:00:00 GMT</pubDate></item></channel></rss>`
	encoded, err := charmap.Windows1250.NewEncoder().String(feed)
	if err != nil {
		t.Fatal(err)
	}
	_, raw, err := parseRSS([]byte(encoded))
	if err != nil || len(raw) != 1 || raw[0].title != "Příliš žluťoučký kůň úpěl ďábelské ódy" {
		t.Errorf("%+v, %v", raw, err)
	}
}

// bodyTransport answers every request with one body.
type bodyTransport string

func (b bodyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(string(b))),
		Request: req}, nil
}

// Some sites answer a wrong address with their default feed, an empty one or a page.
func TestReadFeedChecksTheChannel(t *testing.T) {
	ct24 := feedNamed(t, "ČT24")
	for name, body := range map[string]string{
		"another feed": string(readBulletinFixture(t, "novinky.xml")),
		"empty":        `<rss version="2.0"><channel><title>ČT24</title></channel></rss>`,
		"a page":       `<!DOCTYPE html><html><head><title>Not found</title></head><body><p>Oops</body></html>`,
	} {
		f := newFetcher(&http.Client{Transport: bodyTransport(body)})
		if items, err := readFeed(context.Background(), f, ct24); err == nil {
			t.Errorf("%s: %d items and no error", name, len(items))
		}
	}
}

func TestCleanText(t *testing.T) {
	for in, want := range map[string]string{
		"  Two" + string(noBreakSpace) + " spaces\n and\ta tab ": "Two spaces and a tab",
		"<p>Para</p><p>graphs</p>":                               "Para graphs",
		"a < b and c > d":                                        "a < b and c > d",
		"&lt;b&gt;bold&lt;/b&gt; text":                           "<b>bold</b> text",
		"Zero" + string(zeroWidthSpace) + "width":                "Zerowidth",
		"Dash " + string(emDash) + " dash":                       "Dash " + string(enDash) + " dash",
		"&bdquo;Uvozovky&ldquo; a&nbsp;entity &#8222;":           "„Uvozovky“ a entity „",
	} {
		if got := cleanText(in); got != want {
			t.Errorf("cleanText(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestVlnka(t *testing.T) {
	for in, want := range map[string]string{
		"Na pole u Hořovic navezly kamiony":   "Na pole u~Hořovic navezly kamiony",
		"Policie v Praze a v Brně":            "Policie v~Praze a~v~Brně",
		"V Praze i Brně":                      "V~Praze i~Brně",
		"Bez nich (s rodinou) „A přece“":      "Bez nich (s~rodinou) „A~přece“",
		"K soudu, o penězích, z domova":       "K~soudu, o~penězích, z~domova",
		"Končí to a":                          "Končí to a",
		"Vitamín C a D":                       "Vitamín C a~D",
		"Inflace 2,5 % a sazby":               "Inflace 2,5 % a~sazby",
		"Číslo: k. ú. Libeň":                  "Číslo: k. ú. Libeň",
		"Spojenci nás nevarovali, uvedl Zele": "Spojenci nás nevarovali, uvedl Zele",
	} {
		if got := vlnka(in); got != typeset.Replace(want) {
			t.Errorf("vlnka(%q) = %q, want %q", in, got, typeset.Replace(want))
		}
	}
}

func TestNormalizeLink(t *testing.T) {
	for in, want := range map[string]string{
		"https://www.idnes.cz/zpravy/a.A26#utm_source=rss&utm_medium=feed": "https://www.idnes.cz/zpravy/a.A26",
		"https://www.denik.cz/svet/a/?utm_source=rss&utm_medium=feed":      "https://www.denik.cz/svet/a/",
		"https://example.cz/a?id=7&UTM_campaign=x":                         "https://example.cz/a?id=7",
		" HTTPS://WWW.Example.CZ/Path ":                                    "https://www.example.cz/Path",
		"/relative":                                                        "",
		"javascript:alert(1)":                                              "",
		"ftp://example.cz/file":                                            "",
	} {
		if got := normalizeLink(in); got != want {
			t.Errorf("normalizeLink(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseFeedDate(t *testing.T) {
	want := time.Date(2026, 10, 13, 14, 48, 0, 0, time.UTC)
	for _, in := range []string{"Tue, 13 Oct 2026 14:48:00 GMT", "Tue, 13 Oct 2026 16:48:00 +0200", " Tue, 13 Oct 2026 14:48:00 +0000\n",
		"2026-10-13T16:48:00+02:00"} {
		if got, ok := parseFeedDate(in); !ok || !got.Equal(want) {
			t.Errorf("%q: %s %v", in, got, ok)
		}
	}
	if got, ok := parseFeedDate("Tue, 6 Oct 2026 16:48:00 +0200"); !ok || got.Day() != 6 {
		t.Errorf("a day without its leading zero: %s %v", got, ok)
	}
	if _, ok := parseFeedDate("včera"); ok {
		t.Error("včera is a date")
	}
}

func TestOffTopic(t *testing.T) {
	for _, l := range []string{"Kultura", "Kultura - Film/TV", "ShowTime", "show-time", "Životní styl", "zivotni-styl",
		"Prima sport", "Komentáře z byznysu", "Názory a analýzy", "Pořady", "Cestování", "Jídlo a pití", "kultura-24"} {
		if !offTopic([]string{"Zprávy", l}) {
			t.Errorf("%q is news", l)
		}
	}
	for _, l := range []string{"Zprávy - Domácí", "Životní prostředí", "Věda", "Sportovní hala", "Krimi", "Stalo se", "svet-16"} {
		if offTopic([]string{l}) {
			t.Errorf("%q is not news", l)
		}
	}
}

func TestParseTrending(t *testing.T) {
	articles, err := parseTrending(readBulletinFixture(t, "ct24-trending.json"))
	if err != nil || len(articles) != 8 {
		t.Fatalf("%d articles, %v", len(articles), err)
	}
	it := articles[1].item()
	if it.key != "378524" || it.title != "Bulharsko hlásí další útok na lodě v Černém moři, jedna se potopila" || !it.trending ||
		it.link != "https://ct24.ceskatelevize.cz/clanek/svet/drony-zasahly-u-pobrezi-bulharska-dve-lode-jedna-se-potopila-oznamil-premier-radev-378524" {
		t.Errorf("article: %+v", it)
	}
	for _, body := range []string{`{"data":{"articles":[]}}`, `{"data":"changed"}`, `<html>`} {
		if _, err := parseTrending([]byte(body)); err == nil {
			t.Errorf("%s: no error", body)
		}
	}
}

func TestMergeNews(t *testing.T) {
	now := time.Date(2026, 10, 13, 15, 0, 0, 0, time.UTC)
	item := func(outlet, key, title string, age time.Duration) newsItem {
		return newsItem{outlet: outlet, key: key, title: title, link: "https://example.cz/" + key, published: now.Add(-age)}
	}
	picked := item(outletCT24, "1", "Old text", time.Hour)
	picked.editorsPick = true
	trending := func(id int64, path string, age time.Duration) trendingArticle {
		var a trendingArticle
		a.ID, a.Title.Main, a.Slug, a.MainSection.Path = id, "Trending", "slug", path
		a.Publication.DatePublished = now.Add(-age).Unix()
		return a
	}
	got := mergeNews([]newsItem{
		item(outletCT24, "1", "New text", time.Hour), picked,
		item(outletIDnes, "1", "Another outlet's article 1", time.Hour),
		item(outletNovinky, "old", "Thirteen hours ago", 13*time.Hour),
		item(outletNovinky, "ahead", "Two hours ahead", -2*time.Hour),
		item(outletNovinky, "skew", "Half an hour ahead", -30*time.Minute),
	}, []trendingArticle{trending(1, "svet", time.Hour), trending(2, "domaci", 2*time.Hour), trending(3, "kultura", time.Hour),
		trending(4, "svet", 20*time.Hour)}, now)
	var keys []string
	for _, it := range got {
		keys = append(keys, it.outlet+"/"+it.key)
	}
	if want := []string{"ct24/1", "idnes/1", "novinky/skew", "ct24/2"}; !slices.Equal(keys, want) {
		t.Fatalf("items %q, want %q", keys, want)
	}
	if got[0].title != "New text" || !got[0].editorsPick || !got[0].trending {
		t.Errorf("merged article: %+v", got[0])
	}
	if a := got[3]; !a.trending || a.link != "https://ct24.ceskatelevize.cz/clanek/domaci/slug-2" {
		t.Errorf("an article only trending: %+v", a)
	}
}

// The ranking's rules on made-up stories, each told by different words.
func TestRankNewsRules(t *testing.T) {
	now := time.Date(2026, 10, 13, 15, 0, 0, 0, time.UTC)
	item := func(outlet, title string, age time.Duration) newsItem {
		return newsItem{outlet: outlet, key: title, title: title, link: "https://example.cz/" + outlet, published: now.Add(-age)}
	}
	pick := func(it newsItem) newsItem { it.editorsPick = true; return it }
	trend := func(it newsItem) newsItem { it.trending = true; return it }
	items := []newsItem{
		// four outlets
		item(outletNovinky, "Povodně na Ostravsku: hasiči evakuují stovky obyvatel", time.Hour),
		item(outletIRozhlas, "Hasiči kvůli povodním evakuovali stovky obyvatel Ostravska", time.Hour),
		item(outletCT24, "Povodně zasáhly Ostravsko, hasiči evakuovali stovky obyvatel", time.Hour),
		item(outletDenik, "Povodně na Ostravsku, hasiči evakuovali stovky obyvatel", time.Hour),
		// ČT24's second article on the floods: not a story of its own
		item(outletCT24, "Povodně na Ostravsku: hasiči evakuovali obyvatele, stovky jich nocují ve školách", 30*time.Minute),
		// two outlets and the editors' selection
		pick(item(outletCT24, "Vláda schválila státní rozpočet se schodkem", 2*time.Hour)),
		item(outletSeznamZpravy, "Vláda schválila rozpočet, schodek státu stoupne", 2*time.Hour),
		// two outlets, trending
		trend(item(outletCT24, "Kometa Leonard přiletí nad Česko, astronomové lákají k dalekohledům", 3*time.Hour)),
		item(outletIDnes, "Nad Českem přiletí kometa Leonard, astronomové radí s dalekohledy", 3*time.Hour),
		// two outlets, no ČT24: iROZHLAS tells it
		item(outletNovinky, "Ceny elektřiny klesnou, dodavatelé zlevňují tarify", time.Hour),
		item(outletIRozhlas, "Dodavatelé zlevňují tarify, ceny elektřiny klesnou", time.Hour),
		// two outlets, fresher and older: the oldest is the sixth story, one too many
		item(outletSeznamZpravy, "Železniční most přes Vltavu uzavřen kvůli opravám", 2*time.Hour),
		item(outletIDnes, "Most přes Vltavu uzavřen, železniční opravy potrvají", 2*time.Hour),
		item(outletSeznamZpravy, "Zoologická zahrada vítá mláďata žiraf", 11*time.Hour),
		item(outletIDnes, "Mláďata žiraf vítá zoologická zahrada", 11*time.Hour),
		// only the outlets used for ranking
		item(outletCNNPrima, "Zemětřesení otřáslo Tureckem, budovy padaly", time.Hour),
		item(outletDenik, "Tureckem otřáslo zemětřesení, budovy padaly", time.Hour),
	}
	got := rankNews(items, now)
	var told []string
	for _, s := range got {
		told = append(told, s.OutletName+": "+strings.ReplaceAll(s.Headline, string(noBreakSpace), " "))
	}
	want := []string{
		"ČT24: Povodně zasáhly Ostravsko, hasiči evakuovali stovky obyvatel",
		"ČT24: Vláda schválila státní rozpočet se schodkem",
		"ČT24: Kometa Leonard přiletí nad Česko, astronomové lákají k dalekohledům",
		"iROZHLAS: Dodavatelé zlevňují tarify, ceny elektřiny klesnou",
		"Seznam Zprávy: Železniční most přes Vltavu uzavřen kvůli opravám",
	}
	if !slices.Equal(told, want) {
		t.Fatalf("stories:\n%q\nwant\n%q", told, want)
	}
	if s := got[0]; !slices.Equal(s.Coverage, []string{outletCT24, outletIRozhlas, outletNovinky, outletDenik}) || s.EditorsPick || s.Trending {
		t.Errorf("the floods: %+v", s)
	}
	// two outlets and the selection, two hours old: (2 + 1.5) halved over a day
	if score := math.Round(3.5*math.Exp2(-2.0/24)*100) / 100; !got[1].EditorsPick || got[1].Score != score || !got[2].Trending {
		t.Errorf("the selection and trending: %+v %+v", got[1], got[2])
	}

	// the same stories whatever order the feeds came in
	shuffled := slices.Clone(items)
	rand.New(rand.NewPCG(1, 2)).Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
	again := rankNews(shuffled, now)
	for i := range got {
		if again[i].Headline != got[i].Headline || again[i].Score != got[i].Score {
			t.Errorf("story %d: %q, then %q", i, got[i].Headline, again[i].Headline)
		}
	}
	if rankNews(nil, now) == nil {
		t.Error("no items rank to nil")
	}
}

// The feeds of 13 Oct 2026 at 17:02, the day the ranking was designed on. An hour and a
// half earlier, with ČTK and Economia (left out here: their terms forbid it), the research's
// prototype led with the Bulgarian ships, the Hungarian law, the waste near Hořovice, ČEZ
// and Diag Human, and the Nuremberg alert; by now Novinky and iDNES no longer carry some of
// them.
func TestRankDemoDay(t *testing.T) {
	fresh, trending, feeds := fetchNews(context.Background(), fixtureFetcher())
	for _, f := range feeds {
		if f.Error != "" || f.Items == 0 {
			t.Errorf("%s: %d items, %q", f.Name, f.Items, f.Error)
		}
	}
	now := time.Date(2026, 10, 13, 15, 2, 0, 0, time.UTC)
	got := rankNews(mergeNews(fresh, trending, now), now)
	type story struct {
		outlet, headline string
		coverage         []string
		pick, trending   bool
	}
	want := []story{
		{"ČT24", "Bulharsko hlásí další útok na lodě v~Černém moři, jedna se potopila",
			[]string{outletCT24, outletSeznamZpravy, outletCNNPrima, outletDenik}, true, true},
		{"ČT24", "ČEZ se bude bránit nařízení k~Diag Human u~lucemburského soudu, uvedl Babiš",
			[]string{outletCT24, outletIRozhlas, outletIDnes, outletDenik}, true, false},
		{"iROZHLAS", "Letadlo do Norimberku doprovázely stíhačky a~policie uzavřela letiště. Piloti omylem nahlásili nouzi",
			[]string{outletIRozhlas, outletSeznamZpravy, outletNovinky, outletCNNPrima, outletDenik}, false, false},
		{"ČT24", "Spojenci nás nevarovali před posledním útokem na Kyjev, uvedl Zelenskyj",
			[]string{outletCT24, outletSeznamZpravy, outletIDnes}, true, false},
		{"ČT24", "Pomoc pro přeživší popravy Pikeovou se značně zpozdila, píše AP",
			[]string{outletCT24, outletCNNPrima}, true, true},
	}
	if len(got) != len(want) {
		t.Fatalf("%d stories: %+v", len(got), got)
	}
	for i, w := range want {
		g := got[i]
		if g.OutletName != w.outlet || g.Headline != typeset.Replace(w.headline) || !slices.Equal(g.Coverage, w.coverage) ||
			g.EditorsPick != w.pick || g.Trending != w.trending {
			t.Errorf("story %d: %s %q %v pick=%v trending=%v\nwant %s %q %v pick=%v trending=%v", i+1, g.OutletName, g.Headline,
				g.Coverage, g.EditorsPick, g.Trending, w.outlet, typeset.Replace(w.headline), w.coverage, w.pick, w.trending)
		}
	}
	for i := 1; i < len(got); i++ {
		if got[i].Score > got[i-1].Score {
			t.Errorf("story %d outscores the one before it", i+1)
		}
	}
}
