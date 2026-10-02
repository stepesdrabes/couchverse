package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"

	"couchverse/internal/app"
	"couchverse/internal/config"
	"couchverse/internal/db"
	"couchverse/internal/server"
)

// Seeded ids (testdata/seed.sql).
const (
	movieID       = "00000000-0000-4000-8000-000000000101"
	seriesID      = "00000000-0000-4000-8000-000000000102"
	draftID       = "00000000-0000-4000-8000-000000000103"
	season1ID     = "00000000-0000-4000-8000-000000000201"
	season2ID     = "00000000-0000-4000-8000-000000000202"
	pilotID       = "00000000-0000-4000-8000-000000000301"
	episode2ID    = "00000000-0000-4000-8000-000000000302"
	episode3ID    = "00000000-0000-4000-8000-000000000303"
	unknownID     = "00000000-0000-4000-8000-000000000999"
	movieFileID   = "00000000-0000-4000-8000-000000000501"
	altAudioID    = "00000000-0000-4000-8000-000000000502"
	hdrFileID     = "00000000-0000-4000-8000-000000000503"
	ladderFileID  = "00000000-0000-4000-8000-000000000504"
	thumbArtID    = "00000000-0000-4000-8000-000000000404"
	movieSubID    = "00000000-0000-4000-8000-000000000701"
	failedRungID  = "00000000-0000-4000-8000-000000000802"
	readyRungID   = "00000000-0000-4000-8000-000000000801"
	memberName    = "nora"
	privateMember = "piet"
)

// apiCase is one request in the conformance run. Cases run in order, so later
// ones may depend on (or destroy) what earlier ones read.
type apiCase struct {
	op     string // operationId it exercises
	as     string // "admin", "nora" or "" for anonymous
	method string
	path   string
	body   any
	status int
}

// TestAPIConformance drives every typed operation against a seeded database and
// validates each JSON response against the OpenAPI document, so the published
// contract cannot drift from what the handlers actually send.
func TestAPIConformance(t *testing.T) {
	env := newTestEnv(t)
	doc := server.OpenAPI()
	ops := operations(doc)

	covered := map[string]bool{}
	for i, c := range apiCases() {
		name := fmt.Sprintf("%02d %s %s %s", i, c.op, c.method, c.path)
		op, ok := ops[c.op]
		if !ok {
			t.Fatalf("%s: unknown operation %q", name, c.op)
		}
		if op.method != c.method || !op.matches(c.path) {
			t.Fatalf("%s: does not match %s %s", name, op.method, op.path)
		}
		covered[c.op] = true
		status, body, contentType := env.do(t, c)
		if status != c.status {
			t.Errorf("%s: status %d, want %d: %s", name, status, c.status, body)
			continue
		}
		schema := op.responseSchema(doc, status)
		if schema == nil {
			continue
		}
		if !strings.HasPrefix(contentType, "application/json") {
			t.Errorf("%s: content type %q, want JSON", name, contentType)
			continue
		}
		var value any
		if err := json.Unmarshal(body, &value); err != nil {
			t.Errorf("%s: invalid JSON: %v", name, err)
			continue
		}
		res := &huma.ValidateResult{}
		huma.Validate(doc.Components.Schemas, schema, huma.NewPathBuffer([]byte{}, 0), huma.ModeReadFromServer, value, res)
		for _, err := range res.Errors {
			t.Errorf("%s: response does not match the spec: %v", name, err)
		}
	}

	for id, op := range ops {
		if op.typed && !covered[id] {
			t.Errorf("operation %s (%s %s) has no conformance case", id, op.method, op.path)
		}
	}
}

func apiCases() []apiCase {
	return []apiCase{
		{op: "getTheme", method: "GET", path: "/theme", status: 200},
		{op: "getMe", as: "nora", method: "GET", path: "/auth/me", status: 200},
		{op: "getMe", method: "GET", path: "/auth/me", status: 401},
		{op: "login", method: "POST", path: "/auth/login", body: map[string]string{"username": "nora", "password": "wrong"}, status: 401},
		{op: "getFeatures", as: "nora", method: "GET", path: "/features", status: 200},

		{op: "listGenres", as: "nora", method: "GET", path: "/genres?lang=cs", status: 200},
		{op: "getHome", as: "nora", method: "GET", path: "/home", status: 200},
		{op: "getHome", as: "admin", method: "GET", path: "/home?lang=cs", status: 200},
		{op: "browseTitles", as: "nora", method: "GET", path: "/titles?kind=series&sort=name", status: 200},
		{op: "browseTitles", as: "nora", method: "GET", path: "/titles?sort=bogus", status: 400},
		{op: "getTitle", as: "nora", method: "GET", path: "/titles/glass-harbor-2025?lang=cs", status: 200},
		{op: "getTitle", as: "nora", method: "GET", path: "/titles/static-bloom-2024", status: 200},
		{op: "getTitle", as: "nora", method: "GET", path: "/titles/unfinished-2026", status: 404},
		{op: "search", as: "nora", method: "GET", path: "/search?q=glass", status: 200},
		{op: "saveProgress", as: "nora", method: "PUT", path: "/progress", body: map[string]any{"titleId": movieID, "positionSeconds": 1900, "durationSeconds": 6720, "watchedSeconds": 10}, status: 204},
		{op: "saveProgress", as: "nora", method: "PUT", path: "/progress", body: map[string]any{"positionSeconds": 1, "durationSeconds": 2}, status: 400},
		{op: "saveProgressBeacon", as: "nora", method: "POST", path: "/progress", body: map[string]any{"episodeId": episode2ID, "positionSeconds": 700, "durationSeconds": 2640}, status: 204},
		{op: "listContinueWatching", as: "nora", method: "GET", path: "/me/continue-watching", status: 200},
		{op: "addToWatchlist", as: "nora", method: "PUT", path: "/me/watchlist/" + seriesID, status: 204},
		{op: "listWatchlist", as: "nora", method: "GET", path: "/me/watchlist", status: 200},
		{op: "removeFromWatchlist", as: "nora", method: "DELETE", path: "/me/watchlist/" + movieID, status: 204},
		{op: "removeFromWatchlist", as: "nora", method: "DELETE", path: "/me/watchlist/not-a-uuid", status: 404},

		{op: "adminListLibrary", as: "admin", method: "GET", path: "/admin/library?type=series&sort=size", status: 200},
		{op: "adminListLibrary", as: "nora", method: "GET", path: "/admin/library", status: 403},
		{op: "adminGetTitle", as: "admin", method: "GET", path: "/admin/titles/" + seriesID, status: 200},
		{op: "adminGetTitle", as: "admin", method: "GET", path: "/admin/titles/" + movieID, status: 200},
		{op: "adminGetTitle", as: "admin", method: "GET", path: "/admin/titles/" + unknownID, status: 404},
		{op: "adminGetTitleStorage", as: "admin", method: "GET", path: "/admin/titles/" + seriesID + "/storage", status: 200},
		{op: "adminGetEpisodeTranslations", as: "admin", method: "GET", path: "/admin/episodes/" + pilotID + "/translations", status: 200},
		{op: "adminCreateTitle", as: "admin", method: "POST", path: "/admin/titles", body: map[string]any{"kind": "movie", "name": "Brand New", "year": 2020}, status: 201},
		{op: "adminCreateTitle", as: "admin", method: "POST", path: "/admin/titles", body: map[string]any{"kind": "film", "name": "x"}, status: 400},
		{op: "adminUpdateTitle", as: "admin", method: "PATCH", path: "/admin/titles/" + movieID, body: map[string]any{"overview": "Changed", "year": nil, "genres": []string{"Comedy"}}, status: 200},
		{op: "adminSetTitleTranslation", as: "admin", method: "PATCH", path: "/admin/titles/" + movieID + "/translations/cs", body: map[string]string{"name": "N", "overview": "O"}, status: 204},
		{op: "adminDeleteTitleLanguage", as: "admin", method: "DELETE", path: "/admin/titles/" + seriesID + "/languages/en", status: 400},
		{op: "adminDeleteTitleLanguage", as: "admin", method: "DELETE", path: "/admin/titles/" + movieID + "/languages/cs", status: 204},
		{op: "adminCreateSeason", as: "admin", method: "POST", path: "/admin/titles/" + seriesID + "/seasons", body: map[string]any{"seasonNumber": 3, "name": "Three"}, status: 201},
		{op: "adminCreateEpisode", as: "admin", method: "POST", path: "/admin/seasons/" + season2ID + "/episodes", body: map[string]any{"episodeNumber": 2, "name": "Two"}, status: 201},
		{op: "adminUpdateEpisode", as: "admin", method: "PATCH", path: "/admin/episodes/" + episode3ID, body: map[string]any{"name": "Renamed"}, status: 200},
		{op: "adminSetEpisodeTranslation", as: "admin", method: "PATCH", path: "/admin/episodes/" + pilotID + "/translations/cs", body: map[string]string{"name": "A", "overview": "B"}, status: 204},
		{op: "adminBulkTitles", as: "admin", method: "POST", path: "/admin/titles/bulk", body: map[string]any{"ids": []string{draftID}, "action": "publish"}, status: 204},
		{op: "adminBulkTitles", as: "admin", method: "POST", path: "/admin/titles/bulk", body: map[string]any{"ids": []string{}, "action": "publish"}, status: 400},
		{op: "adminDeleteEpisode", as: "admin", method: "DELETE", path: "/admin/episodes/" + episode2ID, status: 204},
		{op: "adminDeleteSeason", as: "admin", method: "DELETE", path: "/admin/seasons/" + season2ID, status: 204},
		{op: "adminDeleteTitle", as: "admin", method: "DELETE", path: "/admin/titles/" + draftID, status: 204},
	}
}

type operation struct {
	id, method, path string
	op               *huma.Operation
	// typed operations answer JSON; raw ones (streams, uploads, sockets) are
	// documented but have no response schema to check
	typed bool
}

func operations(doc *huma.OpenAPI) map[string]operation {
	out := map[string]operation{}
	for path, item := range doc.Paths {
		for method, op := range map[string]*huma.Operation{
			"GET": item.Get, "POST": item.Post, "PUT": item.Put, "PATCH": item.Patch, "DELETE": item.Delete,
		} {
			if op == nil {
				continue
			}
			o := operation{id: op.OperationID, method: method, path: path, op: op}
			for status, resp := range op.Responses {
				if strings.HasPrefix(status, "2") && (status == "204" || resp.Content["application/json"] != nil) {
					o.typed = true
				}
			}
			out[op.OperationID] = o
		}
	}
	return out
}

// matches reports whether a concrete request path fits the operation's template.
func (o operation) matches(requestPath string) bool {
	requestPath, _, _ = strings.Cut(requestPath, "?")
	tmpl := strings.Split(strings.Trim(o.path, "/"), "/")
	got := strings.Split(strings.Trim(requestPath, "/"), "/")
	if len(tmpl) != len(got) {
		return false
	}
	for i := range tmpl {
		if strings.Contains(tmpl[i], "{") {
			continue
		}
		if tmpl[i] != got[i] {
			return false
		}
	}
	return true
}

func (o operation) responseSchema(doc *huma.OpenAPI, status int) *huma.Schema {
	resp := o.op.Responses[strconv.Itoa(status)]
	if resp == nil {
		resp = o.op.Responses["default"]
	}
	if resp == nil {
		return nil
	}
	if resp.Ref != "" {
		resp = doc.Components.Responses[strings.TrimPrefix(resp.Ref, "#/components/responses/")]
	}
	if mt := resp.Content["application/json"]; mt != nil {
		return mt.Schema
	}
	return nil
}

type testEnv struct {
	srv     *httptest.Server
	clients map[string]*http.Client
}

// newTestEnv migrates and seeds a throwaway database and serves the real app
// on it. It needs TEST_DATABASE_URL (a Postgres the test may create databases
// on) and skips without it.
func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()

	admin, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	name := fmt.Sprintf("couchverse_test_%d", os.Getpid())
	if _, err := admin.Exec(ctx, "DROP DATABASE IF EXISTS "+name); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
		_ = admin.Close(context.Background())
	})

	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	pool, err := db.Connect(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	cfg := config.Config{AdminUsername: "admin", AdminPassword: "admin", DataDir: t.TempDir(), JobWorkers: 1}
	a, err := app.New(ctx, cfg, pool)
	if err != nil {
		t.Fatalf("app: %v", err)
	}
	t.Cleanup(a.Close)
	seed, err := os.ReadFile("testdata/seed.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(seed)); err != nil {
		t.Fatalf("seed: %v", err)
	}

	env := &testEnv{srv: httptest.NewServer(a.Handler()), clients: map[string]*http.Client{}}
	t.Cleanup(env.srv.Close)
	env.clients[""] = env.srv.Client()
	for _, user := range []string{"admin", memberName} {
		jar, _ := cookiejar.New(nil)
		c := &http.Client{Jar: jar}
		status, body, _ := env.request(t, c, "POST", "/auth/login", map[string]string{"username": user, "password": "admin"})
		if status != http.StatusOK {
			t.Fatalf("login %s: %d %s", user, status, body)
		}
		env.clients[user] = c
	}
	return env
}

func (e *testEnv) do(t *testing.T, c apiCase) (int, []byte, string) {
	return e.request(t, e.clients[c.as], c.method, c.path, c.body)
}

func (e *testEnv) request(t *testing.T, client *http.Client, method, path string, body any) (int, []byte, string) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, e.srv.URL+"/api/v1"+path, reader)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return res.StatusCode, data, res.Header.Get("Content-Type")
}
