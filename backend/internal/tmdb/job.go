package tmdb

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"couchverse/internal/artwork"
	"couchverse/internal/settings"
	"couchverse/internal/store"
)

// FetchJob applies TMDB metadata + artwork to a title.
type FetchJob struct {
	Store    *store.Store
	Settings *settings.Store
	Artwork  *artwork.Service
}

type FetchPayload struct {
	TitleID string `json:"titleId"`
	TmdbID  int    `json:"tmdbId"`
}

func APIKey(ctx context.Context, st *settings.Store) (string, error) {
	raw, err := st.Get(ctx, "tmdb.api_key")
	if err != nil {
		return "", err
	}
	var key string
	if raw != nil {
		_ = json.Unmarshal(raw, &key)
	}
	if key == "" {
		return "", fmt.Errorf("no TMDB API key configured in settings")
	}
	return key, nil
}

func (j *FetchJob) Handle(ctx context.Context, job *store.Job, report func(int)) error {
	var p FetchPayload
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		return err
	}

	key, err := APIKey(ctx, j.Settings)
	if err != nil {
		return err
	}
	client := New(key)

	title, err := j.Store.TitleByID(ctx, p.TitleID)
	if err != nil {
		return err
	}

	details, err := client.Details(ctx, title.Kind, p.TmdbID)
	if err != nil {
		return err
	}
	report(30)

	up := store.TitleUpdate{
		Overview: &details.Overview,
		TmdbID:   &p.TmdbID,
	}
	if details.Name != "" {
		up.Name = &details.Name
		up.SortName = &details.Name
	}
	if details.Year > 0 {
		up.Year = &details.Year
	}
	if details.RuntimeMinutes > 0 {
		up.RuntimeMinutes = &details.RuntimeMinutes
	}
	if len(details.Genres) > 0 {
		up.Genres = &details.Genres
	}
	if _, err := j.Store.UpdateTitle(ctx, title.ID, up); err != nil {
		return err
	}
	// drafts get their scanner-placeholder slug rebuilt from the TMDB name;
	// published titles keep theirs so existing links stay valid
	if title.Status == "draft" && details.Name != "" && details.Name != title.Name {
		year := title.Year
		if details.Year > 0 {
			year = &details.Year
		}
		_, _ = j.Store.RegenerateTitleSlug(ctx, title.ID, details.Name, year)
	}
	report(50)

	if details.PosterPath != "" {
		data, err := client.DownloadImage(ctx, details.PosterPath)
		if err != nil {
			return err
		}
		if _, err := j.Artwork.SaveBytes(ctx, "title", title.ID, "poster", ".jpg", data, "tmdb"); err != nil {
			return err
		}
	}
	report(75)

	if details.BackdropPath != "" {
		data, err := client.DownloadImage(ctx, details.BackdropPath)
		if err != nil {
			return err
		}
		if _, err := j.Artwork.SaveBytes(ctx, "title", title.ID, "backdrop", ".jpg", data, "tmdb"); err != nil {
			return err
		}
	}

	// release date column is informative only; parse quietly
	if details.ReleaseDate != "" {
		if _, err := time.Parse("2006-01-02", details.ReleaseDate); err == nil {
			_ = setReleaseDate(ctx, j.Store, title.ID, details.ReleaseDate)
		}
	}
	return nil
}

func setReleaseDate(ctx context.Context, st *store.Store, titleID string, date string) error {
	return st.SetTitleReleaseDate(ctx, titleID, date)
}
