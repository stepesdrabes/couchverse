package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"couchverse/internal/feature/artwork"
	"couchverse/internal/feature/catalog"
	"couchverse/internal/feature/jobs"
	"couchverse/internal/feature/metadata"
	"couchverse/internal/settings"
)

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
