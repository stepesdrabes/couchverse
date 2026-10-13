package couchtv

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// The finder's tests replay what the catalogs answered for the production library's
// titles (testdata/soundtracks/<title>.json.gz: request URL to answer, trimmed to the
// fields the clients read). COUCHVERSE_SOUNDTRACK_LIVE=1 also asks the catalogs what a
// fixture lacks, politely, and with COUCHVERSE_SOUNDTRACK_RECORD=1 writes the answers a
// run used back.

// fixtureTitles are the production library's titles (plan 3), as the finder sees them
// without TMDB.
var fixtureTitles = []struct {
	file  string
	title SoundtrackTitle
}{
	{"iron-man", SoundtrackTitle{Name: "Iron Man", Year: 2008}},
	{"iron-man-2", SoundtrackTitle{Name: "Iron Man 2", Year: 2010}},
	{"iron-man-3", SoundtrackTitle{Name: "Iron Man 3", Year: 2013}},
	{"pulp-fiction", SoundtrackTitle{Name: "Pulp Fiction", Year: 1994}},
	{"harry-potter-1", SoundtrackTitle{Name: "Harry Potter and the Philosopher's Stone", Year: 2001}},
	{"harry-potter-2", SoundtrackTitle{Name: "Harry Potter and the Chamber of Secrets", Year: 2002}},
	{"harry-potter-3", SoundtrackTitle{Name: "Harry Potter and the Prisoner of Azkaban", Year: 2004}},
	{"harry-potter-4", SoundtrackTitle{Name: "Harry Potter and the Goblet of Fire", Year: 2005}},
	{"harry-potter-5", SoundtrackTitle{Name: "Harry Potter and the Order of the Phoenix", Year: 2007}},
	{"harry-potter-6", SoundtrackTitle{Name: "Harry Potter and the Half-Blood Prince", Year: 2009}},
	{"harry-potter-7", SoundtrackTitle{Name: "Harry Potter and the Deathly Hallows: Part 1", Year: 2010}},
	{"harry-potter-8", SoundtrackTitle{Name: "Harry Potter and the Deathly Hallows: Part 2", Year: 2011}},
	{"the-simpsons", SoundtrackTitle{Name: "The Simpsons", Year: 1989, Series: true}},
	{"south-park", SoundtrackTitle{Name: "South Park", Year: 1997, Series: true}},
	{"futurama", SoundtrackTitle{Name: "Futurama", Year: 1999, Series: true}},
	{"dexter", SoundtrackTitle{Name: "Dexter", Year: 2006, Series: true}},
	{"invincible", SoundtrackTitle{Name: "Invincible", Year: 2021, Series: true}},
	{"shrek", SoundtrackTitle{Name: "Shrek", Year: 2001}},
}

// fixtureTransport answers catalog requests from a title's fixture, or from the catalogs
// themselves when live (recording what they said).
type fixtureTransport struct {
	t    *testing.T
	live bool

	mu      sync.Mutex
	answers map[string]json.RawMessage
	used    map[string]bool
	last    map[string]time.Time
}

func (f *fixtureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	key := req.URL.String()
	f.mu.Lock()
	raw, ok := f.answers[key]
	f.used[key] = true
	f.mu.Unlock()
	if ok {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(raw)), Request: req}, nil
	}
	if !f.live {
		f.t.Errorf("no fixture for %s", key)
		return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("{}")), Request: req}, nil
	}
	// polite to the real catalogs, like the clients' own gates
	every := time.Second
	if strings.Contains(req.URL.Host, "itunes") {
		every = 4 * time.Second
	}
	f.mu.Lock()
	if wait := time.Until(f.last[req.URL.Host].Add(every)); wait > 0 {
		time.Sleep(wait)
	}
	f.last[req.URL.Host] = time.Now()
	f.mu.Unlock()
	res, err := http.DefaultTransport.RoundTrip(req)
	if err != nil || res.StatusCode != http.StatusOK {
		return res, err
	}
	body, err := io.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		return nil, err
	}
	f.mu.Lock()
	f.answers[key] = trimAnswer(body)
	f.mu.Unlock()
	res.Body = io.NopCloser(bytes.NewReader(body))
	return res, nil
}

// trimAnswer keeps the fields the clients read, with short stand-ins for preview URLs.
func trimAnswer(body []byte) json.RawMessage {
	var v any
	if json.Unmarshal(body, &v) != nil {
		return body
	}
	keep := map[string]bool{"results": true, "data": true, "wrapperType": true, "kind": true, "trackId": true,
		"collectionId": true, "trackName": true, "collectionName": true, "artistName": true, "releaseDate": true,
		"trackTimeMillis": true, "previewUrl": true, "primaryGenreName": true, "trackNumber": true, "id": true,
		"title": true, "duration": true, "preview": true, "artist": true, "name": true, "album": true, "genre_id": true,
		"release_date": true, "record_type": true, "tracks": true, "error": true, "message": true, "code": true}
	var walk func(v any) any
	walk = func(v any) any {
		switch x := v.(type) {
		case map[string]any:
			out := map[string]any{}
			for k, val := range x {
				if !keep[k] {
					continue
				}
				switch {
				case k == "previewUrl":
					out[k] = fmt.Sprintf("https://audio-ssl.itunes.apple.com/preview/%v.m4a", x["trackId"])
				case k == "preview" && val != "":
					out[k] = fmt.Sprintf("https://cdnt-preview.dzcdn.net/preview/%v.mp3", x["id"])
				default:
					out[k] = walk(val)
				}
			}
			return out
		case []any:
			for i := range x {
				x[i] = walk(x[i])
			}
			return x
		}
		return v
	}
	out, _ := json.Marshal(walk(v))
	return out
}

func fixturePath(file string) string {
	return filepath.Join("testdata", "soundtracks", file+".json.gz")
}

// fixtureCatalogs are iTunes and Deezer over a title's fixture (or live), without their
// gates: the transport keeps the pace when live.
func fixtureCatalogs(t *testing.T, file string) ([]SoundtrackCatalog, *fixtureTransport) {
	t.Helper()
	f := &fixtureTransport{t: t, live: os.Getenv("COUCHVERSE_SOUNDTRACK_LIVE") == "1", answers: map[string]json.RawMessage{},
		used: map[string]bool{}, last: map[string]time.Time{}}
	if err := readFixture(fixturePath(file), &f.answers); err != nil && !f.live {
		t.Fatal(err)
	}
	client := &http.Client{Transport: f}
	itunes, deezer := NewITunes(), NewDeezer()
	itunes.HTTP, itunes.gate = client, nil
	deezer.HTTP, deezer.gate = client, nil
	return []SoundtrackCatalog{itunes, deezer}, f
}

func readFixture(path string, out any) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	z, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	return json.NewDecoder(z).Decode(out)
}

// record writes the answers the run used as the title's fixture, one per line.
func (f *fixtureTransport) record(file string) error {
	if !f.live || os.Getenv("COUCHVERSE_SOUNDTRACK_RECORD") != "1" {
		return nil
	}
	keys := make([]string, 0, len(f.used))
	for k := range f.used {
		if _, ok := f.answers[k]; ok {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	var b bytes.Buffer
	z, _ := gzip.NewWriterLevel(&b, gzip.BestCompression)
	io.WriteString(z, "{\n")
	for i, k := range keys {
		key, _ := json.Marshal(k)
		fmt.Fprintf(z, "%s: %s", key, f.answers[k])
		if i < len(keys)-1 {
			io.WriteString(z, ",")
		}
		io.WriteString(z, "\n")
	}
	io.WriteString(z, "}\n")
	if err := z.Close(); err != nil {
		return err
	}
	return os.WriteFile(fixturePath(file), b.Bytes(), 0o644)
}

// findFixture runs the finder for a fixture title.
func findFixture(t *testing.T, file string, title SoundtrackTitle) []SoundtrackCandidate {
	t.Helper()
	catalogs, f := fixtureCatalogs(t, file)
	got, err := findSoundtracks(context.Background(), catalogs, title, "")
	if err != nil {
		t.Fatalf("%s: %v", file, err)
	}
	if err := f.record(file); err != nil {
		t.Fatal(err)
	}
	return got
}
