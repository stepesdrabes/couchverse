package couchtv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"couchverse/internal/version"
)

// The soundtrack finder's catalogs (plan 6.3, "Promo audio from the web"): iTunes and
// Deezer answer searches without a key and offer a 30-second preview of every track,
// AAC in MP4 from iTunes, MP3 from Deezer. The previews are meant for promotion; this is
// a private server airing them in its own promos.

// SoundtrackAlbum is an album a catalog offers.
type SoundtrackAlbum struct {
	ID     string
	Name   string
	Artist string
	Year   int // 0 when unknown
	// Soundtrack: the catalog files it under film and TV music
	Soundtrack bool
	// Single: the catalog calls it a single (iTunes says so in the name instead)
	Single bool
}

// SoundtrackTrack is a track a catalog offers, on its album.
type SoundtrackTrack struct {
	Provider string
	ID       string
	Name     string
	Artist   string
	Album    SoundtrackAlbum
	Number   int           // its place on the album; 0 when unknown
	Duration time.Duration // the whole track, not the preview
	Preview  string        // the preview's URL as the catalog gave it
	// Order is its place in the search that found it, 0 first; -1 when only an album
	// listing had it.
	Order int
}

// A SoundtrackCatalog is a music catalog with previews; tests fake one.
type SoundtrackCatalog interface {
	// Name is the catalog in rows and media URLs: itunes or deezer.
	Name() string
	Tracks(ctx context.Context, query string) ([]SoundtrackTrack, error)
	Albums(ctx context.Context, query string) ([]SoundtrackAlbum, error)
	AlbumTracks(ctx context.Context, album SoundtrackAlbum) ([]SoundtrackTrack, error)
	// Preview opens a track's preview clip.
	Preview(ctx context.Context, track SoundtrackTrack) (io.ReadCloser, error)
}

// Catalog names, as the couchtv_soundtracks CHECK constraint lists them.
const (
	CatalogITunes = "itunes"
	CatalogDeezer = "deezer"
)

// previewExt is the file extension of a catalog's previews.
func previewExt(catalog string) string {
	if catalog == CatalogDeezer {
		return ".mp3"
	}
	return ".m4a"
}

// gate spaces one catalog's requests, process-wide: iTunes allows about 20 a minute.
type gate struct {
	every time.Duration
	mu    sync.Mutex
	next  time.Time
}

func (g *gate) wait(ctx context.Context) error {
	if g == nil || g.every == 0 {
		return nil
	}
	g.mu.Lock()
	at := time.Now()
	if at.Before(g.next) {
		at = g.next
	}
	g.next = at.Add(g.every)
	g.mu.Unlock()
	d := time.Until(at)
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// catalogClient is what both catalogs share: a polite HTTP client.
type catalogClient struct {
	// HTTP sends the requests; nil uses a default client (tests route its transport)
	HTTP *http.Client
	gate *gate
	// hosts the previews may come from, so a catalog's answer never points the server
	// at anything else
	previewHosts []string
}

var catalogHTTP = &http.Client{Timeout: 20 * time.Second}

func (c *catalogClient) client() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return catalogHTTP
}

func (c *catalogClient) get(ctx context.Context, u string, out any) error {
	if err := c.gate.wait(ctx); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Couchverse/"+version.Version+" (self-hosted media server)")
	req.Header.Set("Accept", "application/json")
	res, err := c.client().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%s answered %s", req.URL.Host, res.Status)
	}
	return json.NewDecoder(io.LimitReader(res.Body, 8<<20)).Decode(out)
}

// maxPreview caps a preview download; a 30-second clip is well under 2 MB.
const maxPreview = 16 << 20

func (c *catalogClient) open(ctx context.Context, raw string) (io.ReadCloser, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || !c.allowed(u.Hostname()) {
		return nil, fmt.Errorf("not a preview URL: %q", raw)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Couchverse/"+version.Version+" (self-hosted media server)")
	res, err := c.client().Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		res.Body.Close()
		return nil, fmt.Errorf("%s answered %s", u.Host, res.Status)
	}
	return struct {
		io.Reader
		io.Closer
	}{io.LimitReader(res.Body, maxPreview), res.Body}, nil
}

func (c *catalogClient) allowed(host string) bool {
	for _, h := range c.previewHosts {
		if host == h || strings.HasSuffix(host, "."+h) {
			return true
		}
	}
	return false
}

// ITunes is the iTunes Search API.
type ITunes struct{ catalogClient }

// NewITunes keeps well under the API's limit of about 20 requests a minute.
func NewITunes() *ITunes {
	return &ITunes{catalogClient{gate: &gate{every: 4 * time.Second}, previewHosts: []string{"apple.com", "mzstatic.com"}}}
}

func (*ITunes) Name() string { return CatalogITunes }

const itunesAPI = "https://itunes.apple.com"

type itunesResult struct {
	WrapperType      string `json:"wrapperType"`
	Kind             string `json:"kind"`
	TrackID          int64  `json:"trackId"`
	CollectionID     int64  `json:"collectionId"`
	TrackName        string `json:"trackName"`
	CollectionName   string `json:"collectionName"`
	ArtistName       string `json:"artistName"`
	ReleaseDate      string `json:"releaseDate"`
	TrackTimeMillis  int64  `json:"trackTimeMillis"`
	PreviewURL       string `json:"previewUrl"`
	PrimaryGenreName string `json:"primaryGenreName"`
	TrackNumber      int    `json:"trackNumber"`
}

type itunesAnswer struct {
	Results []itunesResult `json:"results"`
}

// itunesSoundtrack reads Apple's genres: "Soundtrack", "Original Score", "TV Soundtrack".
func itunesSoundtrack(genre string) bool {
	g := strings.ToLower(genre)
	return strings.Contains(g, "soundtrack") || strings.Contains(g, "score")
}

func yearOf(date string) int {
	if len(date) < 4 {
		return 0
	}
	y, _ := strconv.Atoi(date[:4])
	return y
}

func (r itunesResult) album() SoundtrackAlbum {
	return SoundtrackAlbum{ID: strconv.FormatInt(r.CollectionID, 10), Name: r.CollectionName, Artist: r.ArtistName,
		Year: yearOf(r.ReleaseDate), Soundtrack: itunesSoundtrack(r.PrimaryGenreName)}
}

func (r itunesResult) track(album SoundtrackAlbum, order int) SoundtrackTrack {
	return SoundtrackTrack{Provider: CatalogITunes, ID: strconv.FormatInt(r.TrackID, 10), Name: r.TrackName,
		Artist: r.ArtistName, Album: album, Number: r.TrackNumber, Duration: time.Duration(r.TrackTimeMillis) * time.Millisecond,
		Preview: r.PreviewURL, Order: order}
}

func itunesSearch(term, entity string, limit int) string {
	v := url.Values{"term": {term}, "media": {"music"}, "entity": {entity}, "limit": {strconv.Itoa(limit)}}
	return itunesAPI + "/search?" + v.Encode()
}

func (c *ITunes) Tracks(ctx context.Context, query string) ([]SoundtrackTrack, error) {
	var a itunesAnswer
	if err := c.get(ctx, itunesSearch(query, "song", 50), &a); err != nil {
		return nil, err
	}
	var out []SoundtrackTrack
	for _, r := range a.Results {
		if r.WrapperType == "track" && r.Kind == "song" && r.PreviewURL != "" {
			// a song found by search carries its own release date, the original one for a
			// compilation's tracks, which is all the album year there is
			out = append(out, r.track(r.album(), len(out)))
		}
	}
	return out, nil
}

func (c *ITunes) Albums(ctx context.Context, query string) ([]SoundtrackAlbum, error) {
	var a itunesAnswer
	if err := c.get(ctx, itunesSearch(query, "album", 25), &a); err != nil {
		return nil, err
	}
	var out []SoundtrackAlbum
	for _, r := range a.Results {
		if r.WrapperType == "collection" {
			out = append(out, r.album())
		}
	}
	return out, nil
}

func (c *ITunes) AlbumTracks(ctx context.Context, album SoundtrackAlbum) ([]SoundtrackTrack, error) {
	v := url.Values{"id": {album.ID}, "entity": {"song"}, "limit": {"200"}}
	var a itunesAnswer
	if err := c.get(ctx, itunesAPI+"/lookup?"+v.Encode(), &a); err != nil {
		return nil, err
	}
	var out []SoundtrackTrack
	for _, r := range a.Results {
		switch {
		case r.WrapperType == "collection":
			album = r.album()
		case r.WrapperType == "track" && r.Kind == "song" && r.PreviewURL != "":
			out = append(out, r.track(album, -1))
		}
	}
	return out, nil
}

func (c *ITunes) Preview(ctx context.Context, t SoundtrackTrack) (io.ReadCloser, error) {
	return c.open(ctx, t.Preview)
}

// Deezer is Deezer's public API.
type Deezer struct{ catalogClient }

// NewDeezer stays far below the API's 50 requests in 5 seconds.
func NewDeezer() *Deezer {
	return &Deezer{catalogClient{gate: &gate{every: time.Second}, previewHosts: []string{"dzcdn.net", "deezer.com"}}}
}

func (*Deezer) Name() string { return CatalogDeezer }

const deezerAPI = "https://api.deezer.com"

// Deezer's film and TV music genres: Films/Games and Film Scores.
var deezerSoundtrackGenres = map[int]bool{173: true, 174: true}

type deezerTrack struct {
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	Duration int    `json:"duration"`
	Preview  string `json:"preview"`
	Artist   struct {
		Name string `json:"name"`
	} `json:"artist"`
	Album struct {
		ID    int64  `json:"id"`
		Title string `json:"title"`
	} `json:"album"`
}

type deezerAlbum struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	GenreID     int    `json:"genre_id"`
	ReleaseDate string `json:"release_date"`
	RecordType  string `json:"record_type"`
	Artist      struct {
		Name string `json:"name"`
	} `json:"artist"`
	Tracks struct {
		Data []deezerTrack `json:"data"`
	} `json:"tracks"`
}

// deezerError is how Deezer answers a failure, with a 200.
type deezerError struct {
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"error"`
}

func (c *Deezer) get(ctx context.Context, path string, v url.Values, out any) error {
	u := deezerAPI + path
	if len(v) > 0 {
		u += "?" + v.Encode()
	}
	var raw json.RawMessage
	if err := c.catalogClient.get(ctx, u, &raw); err != nil {
		return err
	}
	var e deezerError
	if json.Unmarshal(raw, &e) == nil && e.Error != nil {
		return fmt.Errorf("deezer: %s (%s %d)", e.Error.Message, e.Error.Type, e.Error.Code)
	}
	return json.Unmarshal(raw, out)
}

func (a deezerAlbum) album() SoundtrackAlbum {
	return SoundtrackAlbum{ID: strconv.FormatInt(a.ID, 10), Name: a.Title, Artist: a.Artist.Name, Year: yearOf(a.ReleaseDate),
		Soundtrack: deezerSoundtrackGenres[a.GenreID], Single: a.RecordType == "single"}
}

func (t deezerTrack) track(album SoundtrackAlbum, number, order int) SoundtrackTrack {
	return SoundtrackTrack{Provider: CatalogDeezer, ID: strconv.FormatInt(t.ID, 10), Name: t.Title, Artist: t.Artist.Name,
		Album: album, Number: number, Duration: time.Duration(t.Duration) * time.Second, Preview: t.Preview, Order: order}
}

func (c *Deezer) Tracks(ctx context.Context, query string) ([]SoundtrackTrack, error) {
	var a struct {
		Data []deezerTrack `json:"data"`
	}
	if err := c.get(ctx, "/search", url.Values{"q": {query}, "limit": {"50"}}, &a); err != nil {
		return nil, err
	}
	var out []SoundtrackTrack
	for _, t := range a.Data {
		if t.Preview != "" {
			// a search knows the album's name only, not its year or genre
			album := SoundtrackAlbum{ID: strconv.FormatInt(t.Album.ID, 10), Name: t.Album.Title}
			out = append(out, t.track(album, 0, len(out)))
		}
	}
	return out, nil
}

func (c *Deezer) Albums(ctx context.Context, query string) ([]SoundtrackAlbum, error) {
	var a struct {
		Data []deezerAlbum `json:"data"`
	}
	if err := c.get(ctx, "/search/album", url.Values{"q": {query}, "limit": {"25"}}, &a); err != nil {
		return nil, err
	}
	out := make([]SoundtrackAlbum, 0, len(a.Data))
	for _, al := range a.Data {
		out = append(out, al.album())
	}
	return out, nil
}

func (c *Deezer) AlbumTracks(ctx context.Context, album SoundtrackAlbum) ([]SoundtrackTrack, error) {
	var a deezerAlbum
	if err := c.get(ctx, "/album/"+url.PathEscape(album.ID), nil, &a); err != nil {
		return nil, err
	}
	full := a.album()
	var out []SoundtrackTrack
	for i, t := range a.Tracks.Data {
		if t.Preview != "" {
			out = append(out, t.track(full, i+1, -1))
		}
	}
	return out, nil
}

// Preview asks for the track again first: Deezer's preview URLs expire within the hour.
func (c *Deezer) Preview(ctx context.Context, t SoundtrackTrack) (io.ReadCloser, error) {
	var fresh deezerTrack
	if err := c.get(ctx, "/track/"+url.PathEscape(t.ID), nil, &fresh); err != nil {
		return nil, err
	}
	if fresh.Preview == "" {
		return nil, errors.New("deezer has no preview of the track")
	}
	return c.open(ctx, fresh.Preview)
}
