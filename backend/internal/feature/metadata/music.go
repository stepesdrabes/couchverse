package metadata

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"slices"

	"couchverse/internal/settings"
)

// composerJobs are the crew jobs TMDB credits a title's music to.
var composerJobs = map[string]bool{"Original Music Composer": true, "Main Title Theme Composer": true, "Music": true,
	"Composer": true}

// Music reads, in one request, what TMDB knows of a title's music: the composers its crew
// names and the names it was released under in the US and Britain ("Harry Potter and the
// Sorcerer's Stone"), which Couch TV's soundtrack finder matches catalog albums with.
func (c *Client) Music(ctx context.Context, kind string, tmdbID int) (composers, names []string, err error) {
	path := fmt.Sprintf("/movie/%d", tmdbID)
	if kind == "series" {
		path = fmt.Sprintf("/tv/%d", tmdbID)
	}
	type altTitle struct {
		Country string `json:"iso_3166_1"`
		Title   string `json:"title"`
	}
	var raw struct {
		Credits struct {
			Crew []struct {
				Name string `json:"name"`
				Job  string `json:"job"`
			} `json:"crew"`
		} `json:"credits"`
		AlternativeTitles struct {
			Titles  []altTitle `json:"titles"`  // a movie's
			Results []altTitle `json:"results"` // a show's
		} `json:"alternative_titles"`
	}
	if err := c.get(ctx, path, url.Values{"append_to_response": {"credits,alternative_titles"}}, &raw); err != nil {
		return nil, nil, err
	}
	for _, p := range raw.Credits.Crew {
		if composerJobs[p.Job] && p.Name != "" && !slices.Contains(composers, p.Name) {
			composers = append(composers, p.Name)
		}
	}
	for _, t := range append(raw.AlternativeTitles.Titles, raw.AlternativeTitles.Results...) {
		if (t.Country == "US" || t.Country == "GB") && t.Title != "" && !slices.Contains(names, t.Title) {
			names = append(names, t.Title)
		}
	}
	return composers, names, nil
}

// MusicFacts serves Couch TV's soundtrack finder (couchtv.MusicFacts, wired in
// internal/app): Music with the server's TMDB key.
type MusicFacts struct {
	Settings *settings.Store
	// HTTP sends the requests; nil uses the client's default
	HTTP *http.Client
}

func (m *MusicFacts) MusicFacts(ctx context.Context, kind string, tmdbID int) (composers, names []string, err error) {
	key, err := APIKey(ctx, m.Settings)
	if err != nil {
		// a server without a TMDB key searches without its facts
		return nil, nil, nil
	}
	client := New(key)
	if m.HTTP != nil {
		client.HTTP = m.HTTP
	}
	return client.Music(ctx, kind, tmdbID)
}
