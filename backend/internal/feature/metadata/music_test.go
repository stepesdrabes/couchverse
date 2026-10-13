package metadata

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"slices"
	"testing"
)

func TestMusicReadsComposersAndNames(t *testing.T) {
	movie, err := os.ReadFile("testdata/movie_music.json")
	if err != nil {
		t.Fatal(err)
	}
	var asked url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = r.URL.Query()
		switch r.URL.Path {
		case "/3/movie/671":
			_, _ = w.Write(movie)
		case "/3/tv/456":
			_, _ = w.Write([]byte(`{"credits": {"crew": [{"name": "Danny Elfman", "job": "Main Title Theme Composer"}]},
				"alternative_titles": {"results": [{"iso_3166_1": "US", "title": "Simpsons"}]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	target, _ := url.Parse(srv.URL)
	client := &Client{APIKey: "key", HTTP: &http.Client{Transport: toServer{target}}}

	composers, names, err := client.Music(context.Background(), "movie", 671)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(composers, []string{"John Williams"}) {
		t.Errorf("composers %q", composers)
	}
	// the US and British names, each once; never another country's
	if !slices.Equal(names, []string{"Harry Potter and the Sorcerer's Stone", "Harry Potter and the Philosopher's Stone"}) {
		t.Errorf("names %q", names)
	}
	if asked.Get("append_to_response") != "credits,alternative_titles" {
		t.Errorf("asked %v: credits and names come in one request", asked)
	}

	composers, names, err = client.Music(context.Background(), "series", 456)
	if err != nil || !slices.Equal(composers, []string{"Danny Elfman"}) || !slices.Equal(names, []string{"Simpsons"}) {
		t.Errorf("a show's music: %q %q %v", composers, names, err)
	}
}
