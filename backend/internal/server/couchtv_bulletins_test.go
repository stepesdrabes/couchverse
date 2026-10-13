package server_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"couchverse/internal/feature/couchtv"
	"couchverse/internal/feature/jobs"
	"couchverse/internal/server"
	"couchverse/internal/settings"
)

// bulletinSources answers the bulletins' sources with what they sent on 13 Oct 2026 (the
// couchtv package's fixtures), or 404 while down.
type bulletinSources struct {
	down     atomic.Bool
	requests atomic.Int32
}

func (s *bulletinSources) RoundTrip(req *http.Request) (*http.Response, error) {
	s.requests.Add(1)
	file := map[string]string{
		"api.open-meteo.com/v1/forecast":                                          "openmeteo.json",
		"opendata.chmi.cz/meteorology/weather/alerts/cap/":                        "cap-listing.html",
		"ct24.ceskatelevize.cz/rss":                                               "ct24.xml",
		"ct24.ceskatelevize.cz/rss/tema/vyber-redakce-84313":                      "ct24-vyber-redakce.xml",
		"ct24.ceskatelevize.cz/api/trendingArticles":                              "ct24-trending.json",
		"www.irozhlas.cz/rss/irozhlas":                                            "irozhlas.xml",
		"www.seznamzpravy.cz/rss":                                                 "seznamzpravy.xml",
		"www.novinky.cz/rss":                                                      "novinky.xml",
		"servis.idnes.cz/rss.aspx":                                                "idnes.xml",
		"cnn.iprima.cz/rss":                                                       "cnnprima.xml",
		"www.denik.cz/rss/zpravy.xml":                                             "denik.xml",
		"opendata.chmi.cz/meteorology/weather/alerts/cap/alert_cap_50_110758.xml": "cap-wind.xml",
	}[req.URL.Host+req.URL.Path]
	body, err := os.ReadFile(filepath.Join("..", "feature", "couchtv", "testdata", "bulletins", file))
	if s.down.Load() || file == "" || err != nil {
		return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("")), Request: req}, nil
	}
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(string(body))),
		Request: req}, nil
}

// TestCouchTvBulletins runs couchtv_bulletins over the sources as they were on 13 Oct 2026:
// what it stores and the admin API answers (checked against the spec), the runs it queues,
// a startup run that finds the bulletins fresh, sources failing and stories running out
// with the last good bulletin kept, the switch, and the cleanup.
func TestCouchTvBulletins(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	prague, _ := time.LoadLocation("Europe/Prague")
	clock := time.Date(2026, 10, 13, 17, 2, 0, 0, prague)
	sources := &bulletinSources{}
	store := couchtv.NewStore(env.pool)
	b := &couchtv.Bulletins{Store: store, Settings: settings.NewStore(env.pool), Jobs: jobs.NewStore(env.pool),
		HTTP: &http.Client{Transport: sources}, Now: func() time.Time { return clock }}
	run := func(kind string, periodic bool) error {
		t.Helper()
		p := couchtv.BulletinsPayload{Kind: kind}
		if periodic {
			p.At = clock.UTC().Format(time.RFC3339)
		}
		payload, _ := json.Marshal(p)
		return b.Handle(ctx, &jobs.Job{Payload: payload}, func(int) {})
	}
	saveConfig := func(cfg couchtv.CouchTvConfig) {
		t.Helper()
		if status, body, _ := env.request(t, env.clients["admin"], "PUT", "/admin/couchtv", cfg); status != http.StatusOK {
			t.Fatalf("saving the config: %d %s", status, body)
		}
	}
	// the recorded forecast is of the first three cities
	cfg := couchtv.DefaultConfig()
	cfg.Bulletins.Cities = cfg.Bulletins.Cities[:3]
	saveConfig(cfg)

	if err := run(couchtv.BulletinWeather, true); err != nil {
		t.Fatal(err)
	}
	if err := run(couchtv.BulletinNews, true); err != nil {
		t.Fatal(err)
	}
	got := readBulletins(t, env)
	w := got.Weather
	if w == nil || len(w.Cities) != 3 || w.Cities[0].Name != "Prague" || w.Cities[0].Now.Temperature != 14.9 ||
		w.Attribution != couchtv.WeatherAttribution || !w.ValidUntil.Equal(clock.Add(6*time.Hour)) {
		t.Fatalf("weather: %+v", w)
	}
	// ČHMÚ's newest state is of 8 Oct: read, but nothing in it is in force on the 13th
	if w.Warnings == nil || w.Warnings.Items == nil || len(w.Warnings.Items) != 0 ||
		!w.Warnings.IssuedAt.Equal(time.Date(2026, 10, 8, 8, 42, 52, 0, time.UTC)) {
		t.Errorf("warnings: %+v", w.Warnings)
	}
	n := got.News
	if n == nil || len(n.Stories) != 5 || len(n.Feeds) != 9 || !n.FetchedAt.Equal(clock) {
		t.Fatalf("news: %+v", n)
	}
	if s := n.Stories[0]; s.Outlet != "ct24" || !strings.HasPrefix(s.Headline, "Bulharsko hlásí další útok na lodě") || !s.EditorsPick {
		t.Errorf("the top story: %+v", s)
	}

	// the next runs: the weather at half past, the news at the next third of an hour
	rows, err := env.pool.Query(ctx, `SELECT payload->>'kind', run_at FROM jobs WHERE type = $1 AND payload ? 'at'`, couchtv.BulletinsJob)
	if err != nil {
		t.Fatal(err)
	}
	next := map[string]time.Time{}
	for rows.Next() {
		var kind string
		var at time.Time
		if err := rows.Scan(&kind, &at); err != nil {
			t.Fatal(err)
		}
		next[kind] = at
	}
	rows.Close()
	if !next["weather"].Equal(time.Date(2026, 10, 13, 17, 30, 0, 0, prague)) || !next["news"].Equal(time.Date(2026, 10, 13, 17, 10, 0, 0, prague)) {
		t.Errorf("next runs: %v", next)
	}

	// after a restart a few minutes later both are fresh: nothing is fetched
	clock = clock.Add(5 * time.Minute)
	before := sources.requests.Load()
	if err := run(couchtv.BulletinNews, false); err != nil {
		t.Fatal(err)
	}
	if err := run(couchtv.BulletinWeather, false); err != nil {
		t.Fatal(err)
	}
	if sources.requests.Load() != before {
		t.Errorf("a startup run fetched %d times", sources.requests.Load()-before)
	}

	// the sources fail: the runs fail and the last good bulletins stay
	sources.down.Store(true)
	clock = clock.Add(time.Hour)
	if err := run(couchtv.BulletinNews, true); err == nil {
		t.Error("the news run did not fail")
	}
	if err := run(couchtv.BulletinWeather, true); err == nil {
		t.Error("the weather run did not fail")
	}
	if again := readBulletins(t, env); !again.News.FetchedAt.Equal(n.FetchedAt) || !again.Weather.FetchedAt.Equal(w.FetchedAt) {
		t.Errorf("a failed run replaced a bulletin: %s %s", again.News.FetchedAt, again.Weather.FetchedAt)
	}

	// the next day the same feeds hold nothing of the last 12 hours: too few stories to air
	sources.down.Store(false)
	clock = clock.Add(18 * time.Hour)
	if err := run(couchtv.BulletinNews, true); err != nil {
		t.Fatal(err)
	}
	if again := readBulletins(t, env); !again.News.FetchedAt.Equal(n.FetchedAt) {
		t.Errorf("a bulletin without stories replaced the last one: %+v", again.News)
	}

	// switched off: nothing is fetched, the next run is still queued
	cfg.Bulletins.Enabled = false
	saveConfig(cfg)
	before = sources.requests.Load()
	if err := run(couchtv.BulletinWeather, true); err != nil || sources.requests.Load() != before {
		t.Errorf("a switched-off run fetched %d times: %v", sources.requests.Load()-before, err)
	}

	// the cleanup keeps six of each kind, none older than a day
	clock = clock.Add(6 * time.Hour)
	if _, err := env.pool.Exec(ctx, `
		INSERT INTO couchtv_bulletins (kind, fetched_at, valid_until, data)
		SELECT 'news', $1::timestamptz - n * interval '20 minutes', $1::timestamptz, '{}' FROM generate_series(1, 10) n`, clock); err != nil {
		t.Fatal(err)
	}
	if err := couchtv.PruneBulletins(ctx, store, clock); err != nil {
		t.Fatal(err)
	}
	var news, weather int
	if err := env.pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE kind = 'news'), count(*) FILTER (WHERE kind = 'weather')
		FROM couchtv_bulletins`).Scan(&news, &weather); err != nil {
		t.Fatal(err)
	}
	// the weather of the day before is over a day old
	if news != 6 || weather != 0 {
		t.Errorf("kept %d news and %d weather bulletins", news, weather)
	}
}

// readBulletins asks the admin API for the bulletins and checks the answer against the spec.
func readBulletins(t *testing.T, env *testEnv) couchtv.CouchTvBulletins {
	t.Helper()
	status, body, _ := env.request(t, env.clients["admin"], "GET", "/admin/couchtv/bulletins", nil)
	if status != http.StatusOK {
		t.Fatalf("status %d: %s", status, body)
	}
	doc := server.OpenAPI()
	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		t.Fatal(err)
	}
	res := &huma.ValidateResult{}
	huma.Validate(doc.Components.Schemas, operations(doc)["adminGetCouchTvBulletins"].responseSchema(doc, status),
		huma.NewPathBuffer([]byte{}, 0), huma.ModeReadFromServer, value, res)
	for _, err := range res.Errors {
		t.Errorf("the answer does not match the spec: %v", err)
	}
	var out couchtv.CouchTvBulletins
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	return out
}
