package metadata

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
)

// toServer sends every request to a test server, whatever host it names.
type toServer struct{ target *url.URL }

func (s toServer) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.URL.Scheme, req.URL.Host = s.target.Scheme, s.target.Host
	return http.DefaultTransport.RoundTrip(req)
}

func fixtureLogos(t *testing.T, kind string) []Logo {
	t.Helper()
	fixture, err := os.ReadFile("testdata/movie_images.json")
	if err != nil {
		t.Fatal(err)
	}
	var query url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/3/movie/603/images" && r.URL.Path != "/3/tv/603/images" {
			http.NotFound(w, r)
			return
		}
		query = r.URL.Query()
		_, _ = w.Write(fixture)
	}))
	t.Cleanup(srv.Close)
	target, _ := url.Parse(srv.URL)

	client := &Client{APIKey: "key", HTTP: &http.Client{Transport: toServer{target}}}
	logos, err := client.Logos(context.Background(), kind, 603, []string{"en", "cs"})
	if err != nil {
		t.Fatal(err)
	}
	if got := query.Get("include_image_language"); got != "en,cs,null" {
		t.Errorf("include_image_language = %q, want en,cs,null", got)
	}
	if query.Get("api_key") != "key" {
		t.Error("the API key was not sent")
	}
	return logos
}

func TestLogosParsesTheImagesResponse(t *testing.T) {
	logos := fixtureLogos(t, "series")
	if len(logos) != 5 {
		t.Fatalf("got %d logos, want 5", len(logos))
	}
	if l := logos[1]; l.FilePath != "/en-top.png" || l.Lang != "en" || l.VoteAverage != 5.312 || l.VoteCount != 1 {
		t.Errorf("logo parsed as %+v", l)
	}
	if logos[3].Lang != "" {
		t.Errorf("a null language parsed as %q", logos[3].Lang)
	}
}

func TestBestLogo(t *testing.T) {
	logos := fixtureLogos(t, "movie")
	for _, c := range []struct{ lang, want string }{
		// the SVG has more votes but is skipped; the tie on the average goes to more votes
		{"en", "/en-top.png"},
		// the only Czech logo is an SVG, so the language-neutral one stands in
		{"cs", "/neutral.png"},
		{"de", "/neutral.png"},
	} {
		got := bestLogo(logos, c.lang)
		if got == nil || got.FilePath != c.want {
			t.Errorf("bestLogo(%s) = %+v, want %s", c.lang, got, c.want)
		}
	}
	if got := bestLogo(logos[:1], "en"); got != nil {
		t.Errorf("an SVG-only set picked %+v", got)
	}
}
