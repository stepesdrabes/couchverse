package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"couchverse/internal/feature/artwork"
	"couchverse/internal/feature/catalog"
	"couchverse/internal/feature/jobs"
	"couchverse/internal/feature/metadata"
	"couchverse/internal/settings"
)

// Seeded logos (testdata/seed.sql).
const (
	movieLogoEn = "00000000-0000-4000-8000-000000000407"
	movieLogoCs = "00000000-0000-4000-8000-000000000408"
	seriesLogo  = "00000000-0000-4000-8000-000000000409"
)

type logoFields struct {
	LogoID     string  `json:"logoId"`
	LogoVer    int64   `json:"logoVer"`
	LogoAspect float64 `json:"logoAspect"`
}

// TestTitleLogos checks which logo the viewer reads hand out: the display
// language's, else the base language's, else one not tied to a language.
func TestTitleLogos(t *testing.T) {
	env := newTestEnv(t)
	for _, c := range []struct {
		path   string
		want   string
		aspect float64
	}{
		{"/titles/glass-harbor-2025?lang=cs", movieLogoCs, 2},
		{"/titles/glass-harbor-2025", movieLogoEn, 2.667},
		{"/titles/glass-harbor-2025?lang=de", movieLogoEn, 2.667},
		{"/titles/static-bloom-2024?lang=cs", seriesLogo, 2.5},
	} {
		var got logoFields
		env.getJSON(t, memberName, c.path, &got)
		if got.LogoID != c.want || got.LogoAspect != c.aspect || got.LogoVer == 0 {
			t.Errorf("%s: logo %+v, want %s at %v", c.path, got, c.want, c.aspect)
		}
	}

	var home struct {
		Featured []struct {
			ID string `json:"id"`
			logoFields
		} `json:"featured"`
	}
	env.getJSON(t, memberName, "/home?lang=cs", &home)
	logos := map[string]string{}
	for _, f := range home.Featured {
		logos[f.ID] = f.LogoID
	}
	if logos[movieID] != movieLogoCs || logos[seriesID] != seriesLogo {
		t.Errorf("featured logos %v", logos)
	}

	if status, body, _ := env.request(t, env.clients["admin"], "DELETE", "/admin/artwork/"+seriesLogo, nil); status != http.StatusNoContent {
		t.Fatalf("delete logo: %d %s", status, body)
	}
	var raw map[string]any
	env.getJSON(t, memberName, "/titles/static-bloom-2024", &raw)
	if _, ok := raw["logoId"]; ok {
		t.Errorf("a title without a logo has logoId %v", raw["logoId"])
	}
}

// TestApplyMetadataLogos runs the TMDB jobs against a fake TMDB: apply stores
// a logo per content language and drops stale TMDB ones, import fills only the
// languages without a logo.
func TestApplyMetadataLogos(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	fakeTMDB(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/3/movie/1001":
			fmt.Fprint(w, `{"title": "Glass Harbor", "overview": "A door in the sea."}`)
		case "/3/movie/1001/images":
			if got := r.URL.Query().Get("include_image_language"); got != "en,cs,null" {
				http.Error(w, "include_image_language "+got, http.StatusBadRequest)
				return
			}
			fmt.Fprint(w, `{"logos": [
				{"iso_639_1": "en", "file_path": "/en.svg", "vote_average": 9},
				{"iso_639_1": "en", "file_path": "/en.png", "vote_average": 5},
				{"iso_639_1": null, "file_path": "/neutral.png", "vote_average": 1}]}`)
		case "/3/tv/2002":
			fmt.Fprint(w, `{"seasons": []}`)
		case "/3/tv/2002/images":
			fmt.Fprint(w, `{"logos": [{"iso_639_1": "en", "file_path": "/bloom.png"}]}`)
		case "/t/p/original/en.png":
			_, _ = w.Write(pngOfSize(400, 100))
		case "/t/p/original/neutral.png", "/t/p/original/bloom.png":
			_, _ = w.Write(pngOfSize(300, 100))
		default:
			http.NotFound(w, r)
		}
	}))

	set := settings.NewStore(env.pool)
	if err := set.Set(ctx, "tmdb.api_key", json.RawMessage(`"key"`)); err != nil {
		t.Fatal(err)
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO artwork (owner_kind, owner_id, kind, lang, path, source) VALUES
		('title', $1, 'logo', 'de', 'artwork/stale.png', 'tmdb'),
		('title', $1, 'logo', 'fr', 'artwork/mine.png', 'uploaded')`, movieID); err != nil {
		t.Fatal(err)
	}
	dataDir := t.TempDir()
	art := &artwork.Service{Store: artwork.NewStore(env.pool), DataDir: dataDir}
	cat := catalog.NewStore(env.pool)

	payload, _ := json.Marshal(metadata.FetchPayload{TitleID: movieID, TmdbID: 1001})
	fetch := &metadata.FetchJob{Catalog: cat, Settings: set, Artwork: art}
	if err := fetch.Handle(ctx, &jobs.Job{Payload: payload}, func(int) {}); err != nil {
		t.Fatal(err)
	}
	checkLogos(t, env, dataDir, movieID, []string{
		"cs artwork/title/" + movieID + "/logo-cs.png 300x100 tmdb",
		"en artwork/title/" + movieID + "/logo-en.png 400x100 tmdb",
		"fr artwork/mine.png 0x0 uploaded",
	})

	payload, _ = json.Marshal(metadata.ImportEpisodesPayload{TitleID: seriesID})
	imp := &metadata.ImportEpisodesJob{Catalog: cat, Settings: set, Artwork: art}
	if err := imp.Handle(ctx, &jobs.Job{Payload: payload}, func(int) {}); err != nil {
		t.Fatal(err)
	}
	checkLogos(t, env, dataDir, seriesID, []string{
		"en artwork/title/" + seriesID + "/logo-en.png 300x100 tmdb",
		"- artwork/bloom-logo.png 1000x400 uploaded",
	})
}

// TestLogoBackfill runs the TMDB logo backfill against a fake TMDB: linked titles get the logos
// they lack in their content languages and English (an oversized one stored within bounds),
// uploaded logos and a logo not tied to a language are kept, each title is asked about once
// whether TMDB has a logo for it or not, nothing is asked without a key, and an outage ends a
// run after three titles.
func TestLogoBackfill(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	var mu sync.Mutex
	asked, langs := map[string]int{}, map[string]string{}
	down := false
	// requests are what TMDB was asked since the last call (path: times), outage sets it down
	requests := func() map[string]int {
		mu.Lock()
		defer mu.Unlock()
		out := maps.Clone(asked)
		clear(asked)
		return out
	}
	outage := func(on bool) {
		mu.Lock()
		down = on
		mu.Unlock()
	}
	fakeTMDB(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		asked[r.URL.Path]++
		if down {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		langs[r.URL.Path] = r.URL.Query().Get("include_image_language")
		switch r.URL.Path {
		case "/3/movie/3001/images":
			fmt.Fprint(w, `{"logos": [{"iso_639_1": "cs", "file_path": "/paper-cs.png"},
				{"iso_639_1": "en", "file_path": "/paper-en.png"}, {"iso_639_1": "en", "file_path": "/paper-en.svg", "vote_average": 9}]}`)
		case "/3/tv/3002/images":
			fmt.Fprint(w, `{"logos": [{"iso_639_1": "en", "file_path": "/legacy.png"}]}`)
		case "/3/movie/3003/images":
			fmt.Fprint(w, `{"logos": []}`)
		case "/3/movie/3004/images":
			fmt.Fprint(w, `{"logos": [{"iso_639_1": "cs", "file_path": "/kept-cs.png"}, {"iso_639_1": null, "file_path": "/kept.png"}]}`)
		case "/t/p/original/paper-en.png":
			_, _ = w.Write(opaquePNG(4000, 1000))
		case "/t/p/original/paper-cs.png", "/t/p/original/legacy.png", "/t/p/original/kept.png":
			_, _ = w.Write(opaquePNG(600, 200))
		default:
			http.NotFound(w, r) // 3005: TMDB no longer knows it
		}
	}))
	linked := map[string]string{"paper": "00000000-0000-4000-8000-000000000911", "legacy": "00000000-0000-4000-8000-000000000912",
		"none": "00000000-0000-4000-8000-000000000913", "kept": "00000000-0000-4000-8000-000000000914",
		"stale": "00000000-0000-4000-8000-000000000915"}
	if _, err := env.pool.Exec(ctx, `INSERT INTO titles (id, kind, name, slug, tmdb_id, metadata_languages) VALUES
		($1, 'movie', 'Papírový měsíc', 'paper-moon', 3001, '{cs}'), ($2, 'series', 'Legacy Show', 'legacy-show', 3002, '{}'),
		($3, 'movie', 'Nothing Logo', 'nothing-logo', 3003, '{en}'), ($4, 'movie', 'Kept', 'kept', 3004, '{cs,en}'),
		($5, 'movie', 'Stale Link', 'stale-link', 3005, '{en}')`,
		linked["paper"], linked["legacy"], linked["none"], linked["kept"], linked["stale"]); err != nil {
		t.Fatal(err)
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO artwork (owner_kind, owner_id, kind, lang, path, width, height, source)
		VALUES ('title', $1, 'logo', 'cs', 'artwork/kept-cs.png', 500, 200, 'uploaded')`, linked["kept"]); err != nil {
		t.Fatal(err)
	}

	dataDir := t.TempDir()
	set := settings.NewStore(env.pool)
	backfill := &metadata.LogoBackfill{Catalog: catalog.NewStore(env.pool), Settings: set,
		Artwork: &artwork.Service{Store: artwork.NewStore(env.pool), DataDir: dataDir}, Jobs: jobs.NewStore(env.pool)}
	run := func() error {
		t.Helper()
		return backfill.Handle(ctx, &jobs.Job{}, func(int) {})
	}
	checked := func() int {
		t.Helper()
		var n int
		if err := env.pool.QueryRow(ctx, `SELECT count(*) FROM titles WHERE logos_version = $1`, metadata.LogosVersion).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	if err := run(); err != nil || len(requests()) > 0 || checked() > 0 {
		t.Fatalf("without a key: %v, %d titles checked", err, checked())
	}
	if err := set.Set(ctx, "tmdb.api_key", json.RawMessage(`"key"`)); err != nil {
		t.Fatal(err)
	}

	// TMDB down: the seeded titles need nothing from it, three of the others ask and the
	// run ends there, for the job's retries
	outage(true)
	if err := run(); err == nil {
		t.Error("an outage ended the run without an error")
	}
	if asked := requests(); len(asked) != 3 || checked() != 2 {
		t.Errorf("during an outage TMDB was asked %v and %d titles were checked", asked, checked())
	}

	outage(false)
	if err := run(); err != nil {
		t.Fatal(err)
	}
	got := requests()
	checkLogos(t, env, dataDir, linked["paper"], []string{
		"cs artwork/title/" + linked["paper"] + "/logo-cs.png 600x200 tmdb",
		"en artwork/title/" + linked["paper"] + "/logo-en.png 2560x640 tmdb",
	})
	checkLogos(t, env, dataDir, linked["legacy"], []string{"- artwork/title/" + linked["legacy"] + "/logo.png 600x200 tmdb"})
	checkLogos(t, env, dataDir, linked["none"], nil)
	// the uploaded Czech logo stays; English takes TMDB's logo not tied to a language
	checkLogos(t, env, dataDir, linked["kept"], []string{
		"cs artwork/kept-cs.png 500x200 uploaded",
		"en artwork/title/" + linked["kept"] + "/logo-en.png 600x200 tmdb",
	})
	// the seeded titles have what they need: the movie a logo in each of its languages, the
	// series its uploaded one, not tied to a language
	var seeded string
	if err := env.pool.QueryRow(ctx, `SELECT string_agg(coalesce(lang, '-') || ' ' || path || ' ' || source, ', ' ORDER BY owner_id, lang)
		FROM artwork WHERE owner_id IN ($1, $2) AND kind = 'logo'`, movieID, seriesID).Scan(&seeded); err != nil {
		t.Fatal(err)
	}
	if want := "cs artwork/glass-logo-cs.png tmdb, en artwork/glass-logo-en.png tmdb, - artwork/bloom-logo.png uploaded"; seeded != want {
		t.Errorf("the seeded logos are %q, want %q", seeded, want)
	}
	mu.Lock()
	for path, want := range map[string]string{"/3/movie/3001/images": "cs,en,null", "/3/tv/3002/images": "en,null",
		"/3/movie/3004/images": "en,null"} {
		if langs[path] != want {
			t.Errorf("%s asked for %q, want %q", path, langs[path], want)
		}
	}
	mu.Unlock()
	for path, n := range got {
		if n != 1 || strings.Contains(path, "/1001/") || strings.Contains(path, "/2002/") {
			t.Errorf("TMDB was asked for %s %d times", path, n)
		}
	}
	// every linked title was looked up, the stale link and the one without logos included
	if n := checked(); n != 7 {
		t.Errorf("%d titles checked, want the 7 linked ones", n)
	}

	if err := run(); err != nil {
		t.Fatal(err)
	}
	if asked := requests(); len(asked) > 0 {
		t.Errorf("a second run asked TMDB %v", asked)
	}
}

func opaquePNG(w, h int) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 255
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

// checkLogos compares a title's logo rows, and that the TMDB ones were written.
func checkLogos(t *testing.T, env *testEnv, dataDir, titleID string, want []string) {
	t.Helper()
	rows, err := env.pool.Query(context.Background(), `SELECT coalesce(lang, '-'), path, width, height, source
		FROM artwork WHERE owner_kind = 'title' AND owner_id = $1 AND kind = 'logo' ORDER BY lang`, titleID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var lang, path, source string
		var w, h int
		if err := rows.Scan(&lang, &path, &w, &h, &source); err != nil {
			t.Fatal(err)
		}
		got = append(got, fmt.Sprintf("%s %s %dx%d %s", lang, path, w, h, source))
		if _, err := os.Stat(filepath.Join(dataDir, path)); source == "tmdb" && err != nil {
			t.Errorf("logo file: %v", err)
		}
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("logos\n got %q\nwant %q", got, want)
	}
}

// fakeTMDB answers the requests the metadata client makes to TMDB's API and
// image hosts with handler, for the rest of the test.
func fakeTMDB(t *testing.T, handler http.Handler) {
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	target, _ := url.Parse(srv.URL)
	orig := http.DefaultTransport
	http.DefaultTransport = tmdbRoute{target: target, next: orig}
	t.Cleanup(func() { http.DefaultTransport = orig })
}

type tmdbRoute struct {
	target *url.URL
	next   http.RoundTripper
}

func (r tmdbRoute) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host == "api.themoviedb.org" || req.URL.Host == "image.tmdb.org" {
		req = req.Clone(req.Context())
		req.URL.Scheme, req.URL.Host = r.target.Scheme, r.target.Host
	}
	return r.next.RoundTrip(req)
}

func pngOfSize(w, h int) []byte {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, w, h))); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func (e *testEnv) getJSON(t *testing.T, as, path string, out any) {
	t.Helper()
	status, body, _ := e.request(t, e.clients[as], "GET", path, nil)
	if status != http.StatusOK {
		t.Fatalf("GET %s: %d %s", path, status, body)
	}
	if err := json.Unmarshal(body, out); err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
}
