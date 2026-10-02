package catalog

import (
	"context"
	"encoding/json"

	"couchverse/internal/feature/artwork"
	"couchverse/internal/feature/auth"
	"couchverse/internal/httpx"
	"couchverse/internal/media"
	"couchverse/internal/settings"
)

type Handlers struct {
	store    *Store
	settings *settings.Store
	artwork  *artwork.Store
}

func NewHandlers(st *Store, set *settings.Store, art *artwork.Store) *Handlers {
	return &Handlers{store: st, settings: set, artwork: art}
}

// FeaturedItem is one slide of the home hero carousel: a title plus its
// backdrop and the viewer's My List state.
type FeaturedItem struct {
	*Title
	BackdropID     *string `json:"backdropId"`
	BackdropVer    int64   `json:"backdropVer,omitempty"`
	BackdropAccent string  `json:"backdropAccent,omitempty"`
	InList         bool    `json:"inList"`
}

// HomeRow is one admin-configured shelf. Continue-watching rows fill
// ContinueWatching; every other kind fills Items.
type HomeRow struct {
	Kind             string         `json:"kind" enum:"continue_watching,recently_added,genre"`
	Label            string         `json:"label"`
	Items            []CardItem     `json:"items"`
	ContinueWatching []ContinueItem `json:"continueWatching"`
}

type Home struct {
	Featured []FeaturedItem `json:"featured"`
	Rows     []HomeRow      `json:"rows"`
}

type homeOutput struct{ Body Home }

func (h *Handlers) Home(ctx context.Context, _ *struct{}) (*homeOutput, error) {
	user := auth.UserFrom(ctx)

	titles, err := h.store.FeaturedTitles(ctx, featuredCount(ctx, h.settings))
	if err != nil {
		return nil, err
	}

	featured := []FeaturedItem{}
	for i := range titles {
		item := FeaturedItem{Title: &titles[i]}
		art, err := h.artwork.ArtworkFor(ctx, "title", titles[i].ID)
		if err != nil {
			return nil, err
		}
		for _, a := range art {
			if a.Kind == "backdrop" {
				id := a.ID
				item.BackdropID = &id
				item.BackdropVer = a.CreatedAt.Unix()
				item.BackdropAccent = a.Accent
			}
		}
		if item.InList, err = h.store.WatchlistHas(ctx, user.ID, titles[i].ID); err != nil {
			return nil, err
		}
		featured = append(featured, item)
	}

	configs, err := h.store.HomeRowConfigs(ctx)
	if err != nil {
		return nil, err
	}

	rows := []HomeRow{}
	for _, cfg := range configs {
		row := HomeRow{Kind: cfg.Kind, Label: cfg.Label, Items: []CardItem{}, ContinueWatching: []ContinueItem{}}
		switch cfg.Kind {
		case "continue_watching":
			row.ContinueWatching, err = h.store.ContinueWatching(ctx, user.ID, 20)
		case "recently_added":
			row.Items, err = h.store.RecentlyAdded(ctx, 20)
		case "genre":
			if cfg.GenreID == nil {
				continue
			}
			row.Items, err = h.store.TitlesByGenre(ctx, *cfg.GenreID, 20)
		default:
			continue
		}
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}

	return &homeOutput{Body: Home{Featured: featured, Rows: rows}}, nil
}

// featuredCount is how many titles the home hero cycles through (default 3).
func featuredCount(ctx context.Context, set *settings.Store) int {
	const def = 3
	raw, err := set.Get(ctx, "home")
	if err != nil || raw == nil {
		return def
	}
	var h struct {
		FeaturedCount int `json:"featuredCount"`
	}
	if json.Unmarshal(raw, &h) != nil || h.FeaturedCount < 1 {
		return def
	}
	if h.FeaturedCount > 10 {
		return 10
	}
	return h.FeaturedCount
}

type browseInput struct {
	Kind  string `query:"kind" enum:"movie,series" doc:"Only movies or only series; all when omitted."`
	Genre string `query:"genre" doc:"English genre name (the genre's stable identity)."`
	Query string `query:"q" doc:"Case-insensitive name filter."`
	Sort  string `query:"sort" enum:"added,name,year" doc:"Ordering; newest first when omitted."`
	Page  int    `query:"page" minimum:"1" default:"1"`
}

type BrowsePage struct {
	Items []CardItem `json:"items"`
	Total int        `json:"total"`
}

type browseOutput struct{ Body BrowsePage }

func (h *Handlers) Browse(ctx context.Context, in *browseInput) (*browseOutput, error) {
	items, total, err := h.store.BrowseTitles(ctx, BrowseFilter{
		Kind:  in.Kind,
		Genre: in.Genre,
		Query: in.Query,
		Sort:  in.Sort,
		Page:  in.Page,
	})
	if err != nil {
		return nil, err
	}
	return &browseOutput{Body: BrowsePage{Items: items, Total: total}}, nil
}

// TitleDetail is a published title as the title page shows it. Seasons and
// EpisodeProgress are empty for movies; Progress is set for movies only.
type TitleDetail struct {
	Title       *Title            `json:"title"`
	InWatchlist bool              `json:"inWatchlist"`
	MediaFiles  []media.MediaFile `json:"mediaFiles"`
	Artwork     []artwork.Artwork `json:"artwork"`
	Seasons     []Season          `json:"seasons"`
	// EpisodeProgress is keyed by episode id.
	EpisodeProgress map[string]EpisodeProgress `json:"episodeProgress"`
	Progress        *EpisodeProgress           `json:"progress,omitempty"`
}

type titleInput struct {
	Slug string `path:"slug"`
}

type titleDetailOutput struct{ Body TitleDetail }

func (h *Handlers) Title(ctx context.Context, in *titleInput) (*titleDetailOutput, error) {
	user := auth.UserFrom(ctx)
	t, err := h.store.TitleBySlug(ctx, in.Slug)
	if err != nil {
		return nil, err
	}
	if t.Status != "published" {
		return nil, httpx.NotFoundError()
	}

	out := TitleDetail{Title: t, Seasons: []Season{}, EpisodeProgress: map[string]EpisodeProgress{}}
	if out.InWatchlist, err = h.store.WatchlistHas(ctx, user.ID, t.ID); err != nil {
		return nil, err
	}
	if out.MediaFiles, err = h.store.MediaFilesForTitle(ctx, t.ID); err != nil {
		return nil, err
	}
	if out.Artwork, err = h.artwork.ArtworkFor(ctx, "title", t.ID); err != nil {
		return nil, err
	}

	if t.Kind == "series" {
		if out.Seasons, err = h.store.SeasonsWithEpisodes(ctx, t.ID); err != nil {
			return nil, err
		}
		if out.EpisodeProgress, err = h.store.EpisodeProgressForTitle(ctx, user.ID, t.ID); err != nil {
			return nil, err
		}
	} else {
		position, duration, err := h.store.ProgressFor(ctx, user.ID, &t.ID, nil)
		if err != nil {
			return nil, err
		}
		out.Progress = &EpisodeProgress{Position: position, Duration: duration}
	}

	return &titleDetailOutput{Body: out}, nil
}

type searchInput struct {
	Query string `query:"q"`
}

type searchOutput struct{ Body *SearchResults }

func (h *Handlers) Search(ctx context.Context, in *searchInput) (*searchOutput, error) {
	res, err := h.store.Search(ctx, in.Query, 12)
	if err != nil {
		return nil, err
	}
	return &searchOutput{Body: res}, nil
}

type genresOutput struct{ Body []Genre }

func (h *Handlers) Genres(ctx context.Context, _ *struct{}) (*genresOutput, error) {
	genres, err := h.store.ListGenres(ctx)
	if err != nil {
		return nil, err
	}
	return &genresOutput{Body: genres}, nil
}
