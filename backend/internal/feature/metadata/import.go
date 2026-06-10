package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"couchverse/internal/feature/catalog"
	"couchverse/internal/feature/jobs"
	"couchverse/internal/settings"
)

// ImportEpisodesJob creates missing seasons/episodes for a show from TMDB,
// filling metadata gaps without touching episodes that already have a file.
type ImportEpisodesJob struct {
	Catalog  *catalog.Store
	Settings *settings.Store
}

type ImportEpisodesPayload struct {
	TitleID string `json:"titleId"`
	Seasons []int  `json:"seasons"` // empty = all (specials excluded unless listed)
}

func (j *ImportEpisodesJob) Handle(ctx context.Context, job *jobs.Job, report func(int)) error {
	var p ImportEpisodesPayload
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		return err
	}

	key, err := APIKey(ctx, j.Settings)
	if err != nil {
		return err
	}
	client := New(key)

	title, err := j.Catalog.TitleByID(ctx, p.TitleID)
	if err != nil {
		return err
	}
	if title.Kind != "series" || title.TmdbID == nil {
		return fmt.Errorf("title %s is not a TMDB-linked series", title.ID)
	}

	all, err := client.SeriesSeasons(ctx, *title.TmdbID)
	if err != nil {
		return err
	}
	wanted := []SeasonInfo{}
	for _, season := range all {
		if len(p.Seasons) > 0 {
			if slices.Contains(p.Seasons, season.SeasonNumber) {
				wanted = append(wanted, season)
			}
		} else if season.SeasonNumber > 0 { // skip specials unless asked for
			wanted = append(wanted, season)
		}
	}
	if len(wanted) == 0 {
		return nil
	}

	hasMedia, err := j.Catalog.EpisodeIDsWithMedia(ctx, title.ID)
	if err != nil {
		return err
	}
	existing, err := j.Catalog.SeasonsWithEpisodes(ctx, title.ID)
	if err != nil {
		return err
	}
	protect := map[[2]int]bool{} // (season, episode) -> has media file
	for _, se := range existing {
		for _, e := range se.Episodes {
			if hasMedia[e.ID] {
				protect[[2]int{se.SeasonNumber, e.EpisodeNumber}] = true
			}
		}
	}

	created := 0
	for i, season := range wanted {
		seasonID, err := j.Catalog.ImportSeasonMeta(ctx, title.ID, season.SeasonNumber, season.Name, season.Overview)
		if err != nil {
			return err
		}
		episodes, err := client.SeasonEpisodes(ctx, *title.TmdbID, season.SeasonNumber)
		if err != nil {
			return err
		}
		for _, ep := range episodes {
			var airDate *time.Time
			if t, perr := time.Parse("2006-01-02", ep.AirDate); perr == nil {
				airDate = &t
			}
			var runtime *int
			if ep.RuntimeMinutes > 0 {
				runtime = &ep.RuntimeMinutes
			}
			inserted, err := j.Catalog.ImportEpisodeMeta(ctx, seasonID, ep.EpisodeNumber,
				ep.Name, ep.Overview, airDate, runtime,
				protect[[2]int{season.SeasonNumber, ep.EpisodeNumber}])
			if err != nil {
				return err
			}
			if inserted {
				created++
			}
		}
		report((i + 1) * 100 / len(wanted))
	}
	slog.Info("tmdb episode import finished",
		"titleId", title.ID, "seasons", len(wanted), "episodesCreated", created)
	return nil
}
