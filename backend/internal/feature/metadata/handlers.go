package metadata

import (
	"context"
	"net/http"

	"couchverse/internal/feature/catalog"
	"couchverse/internal/feature/jobs"
	"couchverse/internal/httpx"
	"couchverse/internal/settings"
)

type AdminMetadata struct {
	catalog  *catalog.Store
	settings *settings.Store
	jobs     *jobs.Store
}

func NewAdminMetadata(cat *catalog.Store, set *settings.Store, jb *jobs.Store) *AdminMetadata {
	return &AdminMetadata{catalog: cat, settings: set, jobs: jb}
}

// MetadataJob identifies the background job a metadata request queued; its
// progress shows in the admin job list.
type MetadataJob struct {
	JobID int64 `json:"jobId"`
}

type jobOutput struct{ Body MetadataJob }

type searchInput struct {
	Query string `query:"q" required:"true" minLength:"1"`
	Kind  string `query:"kind" required:"true" enum:"movie,series"`
	Lang  string `query:"lang" doc:"Language (ISO 639-1) of the returned names and overviews; TMDB's default (English) when empty."`
}

type searchOutput struct{ Body []TmdbSearchResult }

// Search proxies a TMDB search so the API key never reaches the browser.
func (h *AdminMetadata) Search(ctx context.Context, in *searchInput) (*searchOutput, error) {
	client, err := h.client(ctx)
	if err != nil {
		return nil, err
	}
	results, err := client.Search(ctx, in.Kind, in.Query, in.Lang)
	if err != nil {
		return nil, tmdbError(err)
	}
	return &searchOutput{Body: results}, nil
}

type seasonsInput struct {
	ID   string `path:"id" format:"uuid"`
	Lang string `query:"lang" doc:"Language (ISO 639-1) of the season names and overviews; TMDB's default (English) when empty."`
}

type seasonsOutput struct{ Body []TmdbSeason }

// Seasons previews a linked show's TMDB seasons for the import picker.
func (h *AdminMetadata) Seasons(ctx context.Context, in *seasonsInput) (*seasonsOutput, error) {
	title, err := h.tmdbSeries(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	client, err := h.client(ctx)
	if err != nil {
		return nil, err
	}
	seasons, err := client.SeriesSeasons(ctx, *title.TmdbID, in.Lang)
	if err != nil {
		return nil, tmdbError(err)
	}
	return &seasonsOutput{Body: seasons}, nil
}

// EpisodeImport picks the TMDB seasons to create episodes for.
type EpisodeImport struct {
	Seasons []int `json:"seasons" required:"false" doc:"Season numbers to import; empty imports every season except specials."`
}

type importEpisodesInput struct {
	ID string `path:"id" format:"uuid"`
	// optional: no body imports every season
	Body *EpisodeImport
}

// ImportEpisodes queues the season/episode import job.
func (h *AdminMetadata) ImportEpisodes(ctx context.Context, in *importEpisodesInput) (*jobOutput, error) {
	title, err := h.tmdbSeries(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	var seasons []int
	if in.Body != nil {
		seasons = in.Body.Seasons
	}
	jobID, err := h.jobs.EnqueueJobOnce(ctx, "import_episodes",
		ImportEpisodesPayload{TitleID: title.ID, Seasons: seasons},
		jobs.EnqueueOpts{Priority: 5})
	if err != nil {
		return nil, err
	}
	return &jobOutput{Body: MetadataJob{JobID: jobID}}, nil
}

func (h *AdminMetadata) tmdbSeries(ctx context.Context, titleID string) (*catalog.Title, error) {
	title, err := h.catalog.TitleByID(ctx, titleID)
	if err != nil {
		return nil, err
	}
	if title.Kind != "series" {
		return nil, httpx.BadRequestError("episode import only applies to series")
	}
	if title.TmdbID == nil {
		return nil, httpx.Fail(http.StatusPreconditionFailed, "no_tmdb_id",
			"link this show to TMDB first (Fetch from TMDB)")
	}
	return title, nil
}

// TmdbLink links a title to a TMDB entry, whose metadata and artwork are then
// fetched into it.
type TmdbLink struct {
	TmdbID int `json:"tmdbId" minimum:"1"`
}

type applyMetadataInput struct {
	ID   string `path:"id" format:"uuid"`
	Body TmdbLink
}

// Apply queues the metadata fetch job for a title.
func (h *AdminMetadata) Apply(ctx context.Context, in *applyMetadataInput) (*jobOutput, error) {
	title, err := h.catalog.TitleByID(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	jobID, err := h.jobs.EnqueueJob(ctx, "fetch_metadata",
		FetchPayload{TitleID: title.ID, TmdbID: in.Body.TmdbID}, jobs.EnqueueOpts{Priority: 5})
	if err != nil {
		return nil, err
	}
	return &jobOutput{Body: MetadataJob{JobID: jobID}}, nil
}

func (h *AdminMetadata) client(ctx context.Context) (*Client, error) {
	key, err := APIKey(ctx, h.settings)
	if err != nil {
		return nil, httpx.Fail(http.StatusPreconditionFailed, "no_tmdb_key", err.Error())
	}
	return New(key), nil
}

func tmdbError(err error) error {
	return httpx.Fail(http.StatusBadGateway, "tmdb_error", err.Error())
}
