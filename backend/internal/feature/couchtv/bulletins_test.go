package couchtv

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fixtureFiles answer the bulletins' sources with what they sent on 2026-10-13 (trimmed),
// ČHMÚ's newest message being a recorded one of 8 Oct with two yellow wind warnings.
var fixtureFiles = map[string]string{
	capDir:                             "cap-listing.html",
	capDir + "alert_cap_50_110758.xml": "cap-wind.xml",
	trendingURL:                        "ct24-trending.json",
}

func init() {
	files := []string{"ct24.xml", "ct24-vyber-redakce.xml", "irozhlas.xml", "seznamzpravy.xml", "novinky.xml", "idnes.xml",
		"cnnprima.xml", "denik.xml"}
	for i, f := range newsFeeds {
		fixtureFiles[f.url] = files[i]
	}
}

// bulletinTransport serves fixtureFiles, the forecast for any query, and 404 for the rest.
type bulletinTransport struct{ requests atomic.Int32 }

func (t *bulletinTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.requests.Add(1)
	name := fixtureFiles[req.URL.String()]
	if strings.HasPrefix(req.URL.String(), openMeteoURL+"?") {
		name = "openmeteo.json"
	}
	body, err := os.ReadFile(filepath.Join("testdata", "bulletins", name))
	if name == "" || err != nil {
		return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("")), Request: req}, nil
	}
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(string(body))),
		Request: req}, nil
}

func fixtureFetcher() *fetcher {
	return newFetcher(&http.Client{Transport: &bulletinTransport{}})
}

func TestNextBulletin(t *testing.T) {
	prague, _ := time.LoadLocation("Europe/Prague")
	at := func(day, hour, minute int) time.Time { return time.Date(2026, 10, day, hour, minute, 0, 0, prague) }
	for _, c := range []struct {
		name  string
		now   time.Time
		every time.Duration
		want  time.Time
	}{
		{"news before the day wakes", at(13, 3, 0), 20 * time.Minute, at(13, 5, 30)},
		{"news as the day wakes", at(13, 5, 30), 20 * time.Minute, at(13, 5, 50)},
		{"news between slots", at(13, 17, 2), 20 * time.Minute, at(13, 17, 10)},
		{"news on a slot", at(13, 17, 10), 20 * time.Minute, at(13, 17, 30)},
		{"news after midnight", at(14, 0, 20), 20 * time.Minute, at(14, 0, 30)},
		{"news after the last slot", at(14, 0, 30), 20 * time.Minute, at(14, 5, 30)},
		{"weather in the evening", at(13, 23, 45), time.Hour, at(14, 0, 30)},
		{"weather during the day", at(13, 17, 2), time.Hour, at(13, 17, 30)},
		{"weather at night", at(14, 1, 0), time.Hour, at(14, 5, 30)},
		// the clocks go back at 03:00 on 25 Oct: the day still wakes at 05:30
		{"weather the night the clocks change", at(25, 2, 30), time.Hour, at(25, 5, 30)},
	} {
		if got := nextBulletin(c.now, prague, c.every); !got.Equal(c.want) {
			t.Errorf("%s: next run at %s, want %s", c.name, got.In(prague), c.want)
		}
	}
}

func TestFetcherIsPolite(t *testing.T) {
	var calls atomic.Int32
	var agents, validators []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		agents = append(agents, r.Header.Get("User-Agent"))
		switch r.URL.Path {
		case "/flaky":
			if n == 1 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			_, _ = io.WriteString(w, "fine")
		case "/gone":
			w.WriteHeader(http.StatusNotFound)
		case "/tagged":
			validators = append(validators, r.Header.Get("If-None-Match")+"|"+r.Header.Get("If-Modified-Since"))
			if r.Header.Get("If-None-Match") == `"v1"` {
				w.WriteHeader(http.StatusNotModified)
				return
			}
			w.Header().Set("ETag", `"v1"`)
			w.Header().Set("Last-Modified", "Tue, 13 Oct 2026 13:30:36 GMT")
			_, _ = io.WriteString(w, "tagged body")
		}
	}))
	defer srv.Close()
	f := newFetcher(srv.Client())
	f.retryWait = time.Millisecond
	ctx := context.Background()

	if body, err := f.get(ctx, srv.URL+"/flaky"); err != nil || string(body) != "fine" {
		t.Errorf("a 503 then a 200 gave %q, %v", body, err)
	}
	calls.Store(0)
	_, err := f.get(ctx, srv.URL+"/gone")
	var status *errFetch
	if !errors.As(err, &status) || status.status != http.StatusNotFound || calls.Load() != 1 {
		t.Errorf("a 404 gave %v after %d requests, want it at once", err, calls.Load())
	}
	for range 2 {
		if body, err := f.get(ctx, srv.URL+"/tagged"); err != nil || string(body) != "tagged body" {
			t.Errorf("got %q, %v", body, err)
		}
	}
	if len(validators) != 2 || validators[0] != "|" || validators[1] != `"v1"|Tue, 13 Oct 2026 13:30:36 GMT` {
		t.Errorf("validators sent: %q", validators)
	}
	for _, a := range agents {
		if !strings.HasPrefix(a, "Couchverse/") {
			t.Errorf("User-Agent %q does not name the app", a)
		}
	}

	// a server down for good: the first try and two more
	calls.Store(0)
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer down.Close()
	if _, err := f.get(ctx, down.URL); err == nil || calls.Load() != 1+fetchRetries {
		t.Errorf("a server down answered %v after %d requests", err, calls.Load())
	}
}

func TestBulletinsConfig(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if !cfg.Bulletins.Enabled || len(cfg.Bulletins.Cities) != 12 || cfg.Bulletins.Cities[0].Name != "Praha" {
		t.Errorf("default bulletins: %+v", cfg.Bulletins)
	}
	for _, c := range cfg.Bulletins.Cities {
		if c.Name == "Pardubice" {
			t.Error("Pardubice would collide with Hradec Králové on the map")
		}
	}
	for name, mutate := range map[string]func(b *CouchTvBulletinsConfig){
		"no city":       func(b *CouchTvBulletinsConfig) { b.Cities = nil },
		"too many":      func(b *CouchTvBulletinsConfig) { b.Cities = append(b.Cities, DefaultCities()...) },
		"nameless":      func(b *CouchTvBulletinsConfig) { b.Cities[1].Name = "  " },
		"twice":         func(b *CouchTvBulletinsConfig) { b.Cities[1].Name = "Praha" },
		"outside":       func(b *CouchTvBulletinsConfig) { b.Cities[2].Lat = 52.52 },
		"wrong way out": func(b *CouchTvBulletinsConfig) { b.Cities[2].Lon = 11.5 },
	} {
		cfg := DefaultConfig()
		mutate(&cfg.Bulletins)
		var invalid *ConfigError
		if err := cfg.Validate(); !errors.As(err, &invalid) || invalid.Code != ErrBulletins {
			t.Errorf("%s: %v", name, err)
		}
	}

	// a config stored before the bulletins existed gets them, switched on
	stored := DefaultConfig()
	stored.Bulletins = CouchTvBulletinsConfig{}
	if got := stored.withDefaults(); !got.Bulletins.Enabled || len(got.Bulletins.Cities) != 12 {
		t.Errorf("an old config reads as %+v", got.Bulletins)
	}
	// an admin's choice stays
	off := DefaultConfig()
	off.Bulletins.Enabled = false
	if off.withDefaults().Bulletins.Enabled {
		t.Error("switched-off bulletins were switched on")
	}
}
