package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"couchverse/internal/config"
	"couchverse/internal/feature/couchtv"
	"couchverse/internal/feature/jobs"
	"couchverse/internal/settings"
)

// fakeCatalog is a music catalog with fixed answers.
type fakeCatalog struct {
	name    string
	tracks  []couchtv.SoundtrackTrack
	preview []byte
	down    bool
}

func (c *fakeCatalog) Name() string { return c.name }

func (c *fakeCatalog) Tracks(context.Context, string) ([]couchtv.SoundtrackTrack, error) {
	if c.down {
		return nil, errors.New("offline")
	}
	return c.tracks, nil
}

func (c *fakeCatalog) Albums(context.Context, string) ([]couchtv.SoundtrackAlbum, error) {
	if c.down {
		return nil, errors.New("offline")
	}
	return nil, nil
}

func (c *fakeCatalog) AlbumTracks(context.Context, couchtv.SoundtrackAlbum) ([]couchtv.SoundtrackTrack, error) {
	return nil, nil
}

func (c *fakeCatalog) Preview(context.Context, couchtv.SoundtrackTrack) (io.ReadCloser, error) {
	if c.down {
		return nil, errors.New("offline")
	}
	return io.NopCloser(bytes.NewReader(c.preview)), nil
}

// previewHosts sends requests for the catalogs' preview hosts to a test server.
type previewHosts struct {
	target *url.URL
	next   http.RoundTripper
}

func (p previewHosts) RoundTrip(req *http.Request) (*http.Response, error) {
	if strings.HasSuffix(req.URL.Host, "apple.com") || strings.HasSuffix(req.URL.Host, "deezer.com") {
		req = req.Clone(req.Context())
		req.URL.Scheme, req.URL.Host = p.target.Scheme, p.target.Host
	}
	return p.next.RoundTrip(req)
}

// TestCouchTvSoundtracks runs the soundtrack finder over a fake catalog on the seeded
// database: the confident match picked, downloaded and measured; its preview played to
// the admin through the server; the promos playing it; an admin's choice kept through a
// search and handed back; a catalog outage changing nothing.
func TestCouchTvSoundtracks(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not installed")
	}
	env := newTestEnv(t, func(cfg *config.Config) { cfg.FFmpegPath = "ffmpeg" })
	ctx := context.Background()

	// a preview as iTunes sends one: AAC, faded in over a second, 30 s
	previewPath := filepath.Join(t.TempDir(), "preview.m4a")
	if out, err := exec.Command("ffmpeg", "-y", "-hide_banner", "-loglevel", "error", "-f", "lavfi",
		"-i", "sine=frequency=330:duration=30", "-af", "afade=t=in:d=1,afade=t=out:st=28:d=2", "-c:a", "aac", previewPath).
		CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg: %v %s", err, out)
	}
	preview, err := os.ReadFile(previewPath)
	if err != nil {
		t.Fatal(err)
	}
	ost := couchtv.SoundtrackAlbum{ID: "11", Name: "Glass Harbor (Original Motion Picture Soundtrack)", Artist: "Ada Lumen",
		Year: 2025, Soundtrack: true}
	track := func(id, name, artist string, album couchtv.SoundtrackAlbum, order int) couchtv.SoundtrackTrack {
		return couchtv.SoundtrackTrack{Provider: couchtv.CatalogITunes, ID: id, Name: name, Artist: artist, Album: album,
			Duration: 3 * time.Minute, Preview: "https://audio-ssl.itunes.apple.com/preview/" + id + ".m4a", Order: order}
	}
	catalog := &fakeCatalog{name: couchtv.CatalogITunes, preview: preview, tracks: []couchtv.SoundtrackTrack{
		track("1001", "Glass Harbor Main Title", "Ada Lumen", ost, 0),
		track("1002", "The Door in the Sea", "Ada Lumen", ost, 1),
		track("1003", "Glass Harbor Theme (Piano Version)", "Piano Covers Club",
			couchtv.SoundtrackAlbum{ID: "12", Name: "Movie Themes on Piano", Year: 2026}, 2),
	}}
	jb := jobs.NewStore(env.pool)
	store := couchtv.NewStore(env.pool)
	finder := &couchtv.Soundtracks{Store: store, Jobs: jb, Catalogs: []couchtv.SoundtrackCatalog{catalog},
		DataDir: env.cfg.DataDir, FFmpegPath: "ffmpeg"}
	run := func(p couchtv.SoundtrackPayload) {
		t.Helper()
		payload, _ := json.Marshal(p)
		if err := finder.Handle(ctx, &jobs.Job{Payload: payload}, func(int) {}); err != nil {
			t.Fatal(err)
		}
	}
	title := func() couchtv.CouchTvTitle {
		t.Helper()
		var out couchtv.CouchTvTitle
		env.getJSON(t, "admin", "/admin/couchtv/titles/"+movieID, &out)
		return out
	}
	candidate := func(ct couchtv.CouchTvTitle, name string) string {
		t.Helper()
		for _, s := range ct.Soundtracks {
			if s.Track == name {
				return s.ID
			}
		}
		t.Fatalf("no candidate %q among %+v", name, ct.Soundtracks)
		return ""
	}

	// the backfill finds the movie, the finder picks its main title
	run(couchtv.SoundtrackPayload{})
	var queued int
	if err := env.pool.QueryRow(ctx, `SELECT count(*) FROM jobs WHERE type = $1 AND payload->>'titleId' = $2`,
		couchtv.SoundtrackJob, movieID).Scan(&queued); err != nil || queued != 1 {
		t.Fatalf("the backfill queued %d searches of the movie (%v)", queued, err)
	}
	run(couchtv.SoundtrackPayload{TitleID: movieID})
	ct := title()
	if len(ct.Soundtracks) != 3 || ct.Soundtracks[0].Track != "Glass Harbor Main Title" || ct.Soundtracks[0].Confidence < 0.7 ||
		ct.Soundtracks[2].Confidence >= 0.7 {
		t.Fatalf("candidates %+v", ct.Soundtracks)
	}
	// a candidate found again keeps its id; the last search's others are gone
	if ct.Soundtracks[0].ID != soundtrackID {
		t.Errorf("the main title's id changed to %s", ct.Soundtracks[0].ID)
	}
	main := ct.Soundtracks[0].ID
	if p := ct.Soundtrack; p == nil || p.SoundtrackID != main || p.Source != "auto" || !p.Ready || ct.PromoBed != "soundtrack" {
		t.Fatalf("the finder's pick %+v, promos play %s", ct.Soundtrack, ct.PromoBed)
	}
	// the excerpt leaves out the fades
	if p := ct.Soundtrack; p.StartMs > 1000 || p.DurationMs < 24_000 || p.StartMs+p.DurationMs > 29_800 {
		t.Errorf("excerpt %d + %d ms", p.StartMs, p.DurationMs)
	}
	if ct.SoundtrackSearchedAt == nil || ct.SoundtrackQuery != "" {
		t.Errorf("searched at %v with %q", ct.SoundtrackSearchedAt, ct.SoundtrackQuery)
	}
	if _, err := os.Stat(filepath.Join(env.cfg.DataDir, "couchtv", "soundtracks", "itunes-1001.m4a")); err != nil {
		t.Errorf("the pick's preview is not kept: %v", err)
	}

	// the admin hears it from the server's copy; one not downloaded yet is fetched first,
	// from the catalog's own host
	status, body, contentType := env.request(t, env.clients["admin"], "GET", "/admin/couchtv/soundtracks/"+main+"/preview", nil)
	if status != http.StatusOK || contentType != "audio/mp4" || !bytes.Equal(body, preview) {
		t.Errorf("preview: %d %s, %d bytes", status, contentType, len(body))
	}
	cdn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/preview/1002.m4a" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(preview)
	}))
	t.Cleanup(cdn.Close)
	target, _ := url.Parse(cdn.URL)
	orig := http.DefaultTransport
	http.DefaultTransport = previewHosts{target: target, next: orig}
	t.Cleanup(func() { http.DefaultTransport = orig })
	door := candidate(ct, "The Door in the Sea")
	status, body, _ = env.request(t, env.clients["admin"], "GET", "/admin/couchtv/soundtracks/"+door+"/preview", nil)
	if status != http.StatusOK || !bytes.Equal(body, preview) {
		t.Errorf("preview fetched on first play: %d, %d bytes", status, len(body))
	}

	// the promos play it: a composition's promo of the movie has the soundtrack as its bed
	beds := composedBeds(t, env, store)
	if len(beds) == 0 || !strings.HasPrefix(beds[0], "/media/v1/soundtrack/itunes/1001/") {
		t.Errorf("the movie's promo beds: %q", beds)
	}

	// an admin's choice is readied by the job, kept through the finder's search
	put := func(body map[string]any) couchtv.CouchTvTitle {
		t.Helper()
		status, raw, _ := env.request(t, env.clients["admin"], "PUT", "/admin/couchtv/titles/"+movieID, body)
		if status != http.StatusOK {
			t.Fatalf("PUT: %d %s", status, raw)
		}
		var out couchtv.CouchTvTitle
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	ct = put(map[string]any{"excluded": false, "soundtrackId": door})
	if p := ct.Soundtrack; p == nil || p.SoundtrackID != door || p.Source != "admin" || p.Ready || !ct.SearchingSoundtrack {
		t.Fatalf("the admin's choice %+v, searching %v", ct.Soundtrack, ct.SearchingSoundtrack)
	}
	run(couchtv.SoundtrackPayload{TitleID: movieID})
	run(couchtv.SoundtrackPayload{TitleID: movieID, Search: true})
	if p := title().Soundtrack; p == nil || p.SoundtrackID != door || p.Source != "admin" || !p.Ready {
		t.Fatalf("after a search the pick is %+v", p)
	}
	// an admin's own query only offers more to choose from
	catalog.tracks = append(catalog.tracks, track("1004", "Harbor Lights", "The Wave Band", ost, 3))
	run(couchtv.SoundtrackPayload{TitleID: movieID, Query: "glass harbor wave band"})
	ct = title()
	candidate(ct, "Harbor Lights")
	if ct.SoundtrackQuery != "glass harbor wave band" || ct.Soundtrack.SoundtrackID != door {
		t.Errorf("after the admin's query: %q, pick %+v", ct.SoundtrackQuery, ct.Soundtrack)
	}

	// back to automatic: the finder's confident best again
	ct = put(map[string]any{"excluded": false})
	if p := ct.Soundtrack; p == nil || p.SoundtrackID != main || p.Source != "auto" {
		t.Fatalf("handed back, the pick is %+v", p)
	}
	run(couchtv.SoundtrackPayload{TitleID: movieID})
	if p := title().Soundtrack; !p.Ready {
		t.Error("the finder's pick is not readied again")
	}

	// an admin's excerpt of the title's own audio comes first
	ct = put(map[string]any{"excluded": false,
		"promoAudio": map[string]any{"mediaFileId": movieFileID, "startMs": 60_000, "durationMs": 30_000}})
	if ct.PromoBed != "excerpt" || ct.Soundtrack == nil {
		t.Errorf("with the admin's excerpt the promos play %s (soundtrack %+v)", ct.PromoBed, ct.Soundtrack)
	}

	// no catalog answers: the title stays as it was and tries again later
	catalog.down = true
	before := title()
	run(couchtv.SoundtrackPayload{TitleID: movieID, Search: true})
	after := title()
	if len(after.Soundtracks) != len(before.Soundtracks) || after.Soundtrack.SoundtrackID != before.Soundtrack.SoundtrackID ||
		!after.SoundtrackSearchedAt.Equal(*before.SoundtrackSearchedAt) {
		t.Errorf("an outage changed the title: %+v", after)
	}
	var retries int
	if err := env.pool.QueryRow(ctx, `SELECT count(*) FROM jobs WHERE type = $1 AND status = 'pending'
		AND (payload->>'retry')::int = 1 AND run_at > now() + interval '1 hour'`, couchtv.SoundtrackJob).Scan(&retries); err != nil || retries != 1 {
		t.Errorf("%d retries queued (%v)", retries, err)
	}
}

// composedBeds composes the breaks of the next day and answers the beds of the movie's
// promos.
func composedBeds(t *testing.T, env *testEnv, store *couchtv.Store) []string {
	t.Helper()
	ctx := context.Background()
	jb, set := jobs.NewStore(env.pool), settings.NewStore(env.pool)
	if err := (&couchtv.Scheduler{Store: store, Settings: set, Jobs: jb}).Handle(ctx, &jobs.Job{}, func(int) {}); err != nil {
		t.Fatal(err)
	}
	versions := `{"ident": 1, "sting": 1, "upNext": 1, "promo": 1, "evergreenEnd": 1, "stripPromo": 1, "shortPromo": 1,
		"lineup": 1, "squeeze": 1, "standby": 1}`
	if err := set.Set(ctx, "couchtv_render", json.RawMessage(`{"versions": `+versions+`, "seenAt": "2026-10-12T08:00:00Z"}`)); err != nil {
		t.Fatal(err)
	}
	if err := (&couchtv.Composer{Store: store, Settings: set, Jobs: jb}).Handle(ctx, &jobs.Job{}, func(int) {}); err != nil {
		t.Fatal(err)
	}
	rows, err := env.pool.Query(ctx, `SELECT DISTINCT spec->'bed'->>'url' FROM couchtv_clips
		WHERE spec->'title'->>'name' = 'Glass Harbor' AND spec->>'template' IN ('promo', 'shortPromo') AND spec ? 'bed'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var beds []string
	for rows.Next() {
		var url string
		if err := rows.Scan(&url); err != nil {
			t.Fatal(err)
		}
		beds = append(beds, url)
	}
	return beds
}
