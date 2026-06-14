package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"couchverse/internal/feature/artwork"
	"couchverse/internal/feature/catalog"
	"couchverse/internal/feature/jobs"
	"couchverse/internal/settings"
)

// ImportEpisodesJob creates missing seasons/episodes for a show from TMDB,
// filling metadata gaps without touching episodes that already have a file.
type ImportEpisodesJob struct {
	Catalog  *catalog.Store
	Settings *settings.Store
	Artwork  *artwork.Service
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

	langs := title.MetadataLanguages
	if len(langs) == 0 {
		langs = []string{""} // legacy series: base/English only
	}
	base := langs[0]

	all, err := client.SeriesSeasons(ctx, *title.TmdbID, base)
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
	seasonIDs := map[int]string{}     // season number -> id (from the base pass)
	episodeIDs := map[[2]int]string{} // (season, episode) -> id
	for i, season := range wanted {
		seasonID, err := j.Catalog.ImportSeasonMeta(ctx, title.ID, season.SeasonNumber, season.Name, season.Overview)
		if err != nil {
			return err
		}
		seasonIDs[season.SeasonNumber] = seasonID
		episodes, err := client.SeasonEpisodes(ctx, *title.TmdbID, season.SeasonNumber, base)
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
			episodeID, inserted, err := j.Catalog.ImportEpisodeMeta(ctx, seasonID, ep.EpisodeNumber,
				ep.Name, ep.Overview, airDate, runtime,
				protect[[2]int{season.SeasonNumber, ep.EpisodeNumber}])
			if err != nil {
				return err
			}
			if inserted {
				created++
			}
			episodeIDs[[2]int{season.SeasonNumber, ep.EpisodeNumber}] = episodeID
			j.importStill(ctx, client, episodeID, ep.StillPath)
		}
		report((i + 1) * 100 / len(wanted))
	}

	// translation passes: fill season/episode translations for the other
	// configured languages, matched to the base pass by number.
	for _, lang := range langs[1:] {
		if lang == "" || lang == base {
			continue
		}
		tseasons, terr := client.SeriesSeasons(ctx, *title.TmdbID, lang)
		if terr != nil {
			return terr
		}
		byNum := map[int]SeasonInfo{}
		for _, ts := range tseasons {
			byNum[ts.SeasonNumber] = ts
		}
		for _, season := range wanted {
			sid, ok := seasonIDs[season.SeasonNumber]
			if !ok {
				continue
			}
			if ts, ok := byNum[season.SeasonNumber]; ok {
				if err := j.Catalog.SetSeasonTranslation(ctx, sid, lang, ts.Name, ts.Overview); err != nil {
					return err
				}
			}
			teps, eerr := client.SeasonEpisodes(ctx, *title.TmdbID, season.SeasonNumber, lang)
			if eerr != nil {
				return eerr
			}
			for _, ep := range teps {
				eid, ok := episodeIDs[[2]int{season.SeasonNumber, ep.EpisodeNumber}]
				if !ok {
					continue
				}
				if err := j.Catalog.SetEpisodeTranslation(ctx, eid, lang, ep.Name, ep.Overview); err != nil {
					return err
				}
			}
		}
	}
	slog.Info("tmdb episode import finished",
		"titleId", title.ID, "seasons", len(wanted), "episodesCreated", created)
	return nil
}

// importStill downloads a TMDB episode still as the episode's thumb artwork,
// skipping episodes that already have one. Best effort - a failure never fails
// the import.
func (j *ImportEpisodesJob) importStill(ctx context.Context, client *Client, episodeID, stillPath string) {
	if stillPath == "" || j.Artwork == nil {
		return
	}
	if existing, err := j.Artwork.Store.ArtworkFor(ctx, "episode", episodeID); err == nil {
		for _, a := range existing {
			if a.Kind == "thumb" {
				return
			}
		}
	}
	data, err := client.DownloadImage(ctx, stillPath)
	if err != nil {
		slog.Warn("tmdb episode still download", "episodeId", episodeID, "err", err)
		return
	}
	if _, err := j.Artwork.SaveBytes(ctx, "episode", episodeID, "thumb", ".jpg", data, "tmdb"); err != nil {
		slog.Warn("save episode still", "episodeId", episodeID, "err", err)
	}
}
