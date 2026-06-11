// Package metadata fetches title metadata and artwork from TMDB.
package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	apiBase   = "https://api.themoviedb.org/3"
	imageBase = "https://image.tmdb.org/t/p"
)

type Client struct {
	APIKey string
	HTTP   *http.Client
}

func New(apiKey string) *Client {
	return &Client{
		APIKey: apiKey,
		HTTP:   &http.Client{Timeout: 15 * time.Second},
	}
}

type SearchResult struct {
	TmdbID    int    `json:"tmdbId"`
	Name      string `json:"name"`
	Year      int    `json:"year"`
	Overview  string `json:"overview"`
	PosterURL string `json:"posterUrl"`
}

type Details struct {
	Name           string
	Overview       string
	Year           int
	ReleaseDate    string
	RuntimeMinutes int
	Genres         []string
	PosterPath     string
	BackdropPath   string
}

func (c *Client) get(ctx context.Context, path string, params url.Values, out any) error {
	params.Set("api_key", c.APIKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBase+path+"?"+params.Encode(), nil)
	if err != nil {
		return err
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("TMDB rejected the API key")
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("TMDB responded with %s", res.Status)
	}
	return json.NewDecoder(res.Body).Decode(out)
}

// Search queries movies or TV shows. kind is "movie" or "series".
func (c *Client) Search(ctx context.Context, kind, query string) ([]SearchResult, error) {
	endpoint := "/search/movie"
	if kind == "series" {
		endpoint = "/search/tv"
	}

	var raw struct {
		Results []struct {
			ID           int    `json:"id"`
			Title        string `json:"title"`
			Name         string `json:"name"`
			ReleaseDate  string `json:"release_date"`
			FirstAirDate string `json:"first_air_date"`
			Overview     string `json:"overview"`
			PosterPath   string `json:"poster_path"`
		} `json:"results"`
	}
	if err := c.get(ctx, endpoint, url.Values{"query": {query}}, &raw); err != nil {
		return nil, err
	}

	results := []SearchResult{}
	for _, r := range raw.Results {
		name := r.Title
		date := r.ReleaseDate
		if kind == "series" {
			name = r.Name
			date = r.FirstAirDate
		}
		res := SearchResult{TmdbID: r.ID, Name: name, Overview: r.Overview}
		if len(date) >= 4 {
			fmt.Sscanf(date[:4], "%d", &res.Year)
		}
		if r.PosterPath != "" {
			res.PosterURL = imageBase + "/w185" + r.PosterPath
		}
		results = append(results, res)
		if len(results) == 10 {
			break
		}
	}
	return results, nil
}

func (c *Client) Details(ctx context.Context, kind string, tmdbID int) (*Details, error) {
	endpoint := fmt.Sprintf("/movie/%d", tmdbID)
	if kind == "series" {
		endpoint = fmt.Sprintf("/tv/%d", tmdbID)
	}

	var raw struct {
		Title        string `json:"title"`
		Name         string `json:"name"`
		Overview     string `json:"overview"`
		ReleaseDate  string `json:"release_date"`
		FirstAirDate string `json:"first_air_date"`
		Runtime      int    `json:"runtime"`
		Genres       []struct {
			Name string `json:"name"`
		} `json:"genres"`
		PosterPath   string `json:"poster_path"`
		BackdropPath string `json:"backdrop_path"`
	}
	if err := c.get(ctx, endpoint, url.Values{}, &raw); err != nil {
		return nil, err
	}

	d := &Details{
		Name:           raw.Title,
		Overview:       raw.Overview,
		ReleaseDate:    raw.ReleaseDate,
		RuntimeMinutes: raw.Runtime,
		PosterPath:     raw.PosterPath,
		BackdropPath:   raw.BackdropPath,
	}
	if kind == "series" {
		d.Name = raw.Name
		d.ReleaseDate = raw.FirstAirDate
	}
	if len(d.ReleaseDate) >= 4 {
		fmt.Sscanf(d.ReleaseDate[:4], "%d", &d.Year)
	}
	for _, g := range raw.Genres {
		d.Genres = append(d.Genres, g.Name)
	}
	return d, nil
}

type SeasonInfo struct {
	SeasonNumber int    `json:"seasonNumber"`
	Name         string `json:"name"`
	Overview     string `json:"overview"`
	EpisodeCount int    `json:"episodeCount"`
}

type EpisodeInfo struct {
	EpisodeNumber  int
	Name           string
	Overview       string
	AirDate        string // YYYY-MM-DD or ""
	RuntimeMinutes int
	StillPath      string // TMDB still image path, "" when absent
}

// SeriesSeasons lists a show's seasons (including specials/season 0).
func (c *Client) SeriesSeasons(ctx context.Context, tmdbID int) ([]SeasonInfo, error) {
	var raw struct {
		Seasons []struct {
			SeasonNumber int    `json:"season_number"`
			Name         string `json:"name"`
			Overview     string `json:"overview"`
			EpisodeCount int    `json:"episode_count"`
		} `json:"seasons"`
	}
	if err := c.get(ctx, fmt.Sprintf("/tv/%d", tmdbID), url.Values{}, &raw); err != nil {
		return nil, err
	}
	seasons := []SeasonInfo{}
	for _, s := range raw.Seasons {
		seasons = append(seasons, SeasonInfo(s))
	}
	return seasons, nil
}

// SeasonEpisodes lists the episodes of one season.
func (c *Client) SeasonEpisodes(ctx context.Context, tmdbID, season int) ([]EpisodeInfo, error) {
	var raw struct {
		Episodes []struct {
			EpisodeNumber int    `json:"episode_number"`
			Name          string `json:"name"`
			Overview      string `json:"overview"`
			AirDate       string `json:"air_date"`
			Runtime       int    `json:"runtime"`
			StillPath     string `json:"still_path"`
		} `json:"episodes"`
	}
	if err := c.get(ctx, fmt.Sprintf("/tv/%d/season/%d", tmdbID, season), url.Values{}, &raw); err != nil {
		return nil, err
	}
	episodes := []EpisodeInfo{}
	for _, e := range raw.Episodes {
		episodes = append(episodes, EpisodeInfo{
			EpisodeNumber:  e.EpisodeNumber,
			Name:           e.Name,
			Overview:       e.Overview,
			AirDate:        e.AirDate,
			RuntimeMinutes: e.Runtime,
			StillPath:      e.StillPath,
		})
	}
	return episodes, nil
}

// DownloadImage fetches a TMDB image (poster_path/backdrop_path) at original size.
func (c *Client) DownloadImage(ctx context.Context, imagePath string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageBase+"/original"+imagePath, nil)
	if err != nil {
		return nil, err
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("image download failed: %s", res.Status)
	}
	return io.ReadAll(io.LimitReader(res.Body, 30<<20))
}
