package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestSPAServesPrecompressedAssets(t *testing.T) {
	dist := fstest.MapFS{
		"index.html":                  {Data: []byte("<html>")},
		"index.html.gz":               {Data: []byte("gz-html")},
		"_app/immutable/core.wasm":    {Data: []byte("wasm")},
		"_app/immutable/core.wasm.br": {Data: []byte("br-wasm")},
		"_app/immutable/core.wasm.gz": {Data: []byte("gz-wasm")},
		"_app/immutable/entry/app.js": {Data: []byte("js")},
		"robots.txt":                  {Data: []byte("robots")},
	}
	handler := serveSPA(dist)
	get := func(path, accept string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if accept != "" {
			req.Header.Set("Accept-Encoding", accept)
		}
		rec := httptest.NewRecorder()
		handler(rec, req)
		return rec
	}

	cases := []struct {
		path, accept, body, encoding, contentType, cache string
	}{
		{"/_app/immutable/core.wasm", "gzip, deflate, br", "br-wasm", "br", "application/wasm", "public, max-age=31536000, immutable"},
		{"/_app/immutable/core.wasm", "gzip", "gz-wasm", "gzip", "application/wasm", "public, max-age=31536000, immutable"},
		{"/_app/immutable/core.wasm", "br;q=0, gzip;q=0.5", "gz-wasm", "gzip", "application/wasm", "public, max-age=31536000, immutable"},
		{"/_app/immutable/core.wasm", "", "wasm", "", "application/wasm", "public, max-age=31536000, immutable"},
		// no compressed sibling: the file as it is
		{"/_app/immutable/entry/app.js", "br", "js", "", "text/javascript; charset=utf-8", "public, max-age=31536000, immutable"},
		// client-side routes get the shell, compressed when possible, never cached
		{"/movies", "gzip", "gz-html", "gzip", "text/html; charset=utf-8", "no-cache"},
		{"/", "", "<html>", "", "text/html; charset=utf-8", "no-cache"},
		{"/robots.txt", "br", "robots", "", "text/plain; charset=utf-8", ""},
	}
	for _, c := range cases {
		rec := get(c.path, c.accept)
		if rec.Code != http.StatusOK || rec.Body.String() != c.body {
			t.Errorf("%s (%q): %d %q, want %q", c.path, c.accept, rec.Code, rec.Body.String(), c.body)
		}
		h := rec.Header()
		if h.Get("Content-Encoding") != c.encoding || h.Get("Content-Type") != c.contentType || h.Get("Cache-Control") != c.cache {
			t.Errorf("%s (%q): encoding %q type %q cache %q", c.path, c.accept, h.Get("Content-Encoding"), h.Get("Content-Type"), h.Get("Cache-Control"))
		}
		if h.Get("Vary") != "Accept-Encoding" {
			t.Errorf("%s: Vary %q", c.path, h.Get("Vary"))
		}
	}
}
