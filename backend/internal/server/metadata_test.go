package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"couchverse/internal/feature/artwork"
	"couchverse/internal/feature/catalog"
	"couchverse/internal/feature/jobs"
	"couchverse/internal/feature/metadata"
	"couchverse/internal/settings"
)

// TestApplyMetadataTaglineAndCollection checks that a TMDB apply keeps the base
// language's tagline and the movie's collection, and that a later apply without a
// collection clears it.
func TestApplyMetadataTaglineAndCollection(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	collection := `, "belongs_to_collection": {"id": 77, "name": "Harbor Collection"}`
	fakeTMDB(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/3/movie/1001":
			if r.URL.Query().Get("language") == "cs" {
				fmt.Fprint(w, `{"title": "Skleněný přístav", "tagline": "Dveře v moři."}`)
				return
			}
			fmt.Fprintf(w, `{"title": "Glass Harbor", "tagline": "Some doors open inward."%s}`, collection)
		case "/3/movie/1001/images":
			fmt.Fprint(w, `{"logos": []}`)
		default:
			http.NotFound(w, r)
		}
	}))
	set := settings.NewStore(env.pool)
	if err := set.Set(ctx, "tmdb.api_key", json.RawMessage(`"key"`)); err != nil {
		t.Fatal(err)
	}
	fetch := &metadata.FetchJob{Catalog: catalog.NewStore(env.pool), Settings: set,
		Artwork: &artwork.Service{Store: artwork.NewStore(env.pool), DataDir: t.TempDir()}}
	payload, _ := json.Marshal(metadata.FetchPayload{TitleID: movieID, TmdbID: 1001})
	apply := func() (tagline string, collectionID *int, collectionName, csTagline string) {
		t.Helper()
		if err := fetch.Handle(ctx, &jobs.Job{Payload: payload}, func(int) {}); err != nil {
			t.Fatal(err)
		}
		if err := env.pool.QueryRow(ctx, `SELECT tagline, collection_tmdb_id, collection_name, translations->'cs'->>'tagline'
			FROM titles WHERE id = $1`, movieID).Scan(&tagline, &collectionID, &collectionName, &csTagline); err != nil {
			t.Fatal(err)
		}
		return
	}

	tagline, id, name, cs := apply()
	if tagline != "Some doors open inward." || id == nil || *id != 77 || name != "Harbor Collection" || cs != "Dveře v moři." {
		t.Errorf("stored tagline %q, collection %v %q, cs tagline %q", tagline, id, name, cs)
	}

	collection = ""
	if _, id, name, _ = apply(); id != nil || name != "" {
		t.Errorf("a movie no longer in a collection keeps %v %q", id, name)
	}

	// dropping the base language promotes the next one's tagline
	if status, body, _ := env.request(t, env.clients["admin"], "DELETE", "/admin/titles/"+movieID+"/languages/en", nil); status != http.StatusNoContent {
		t.Fatalf("drop en: %d %s", status, body)
	}
	if err := env.pool.QueryRow(ctx, `SELECT tagline FROM titles WHERE id = $1`, movieID).Scan(&tagline); err != nil {
		t.Fatal(err)
	}
	if tagline != "Dveře v moři." {
		t.Errorf("after dropping en the tagline is %q", tagline)
	}
}
