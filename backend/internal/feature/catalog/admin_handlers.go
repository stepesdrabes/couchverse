package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"couchverse/internal/feature/artwork"
	"couchverse/internal/feature/jobs"
	"couchverse/internal/httpx"
	"couchverse/internal/media"
)

type AdminHandlers struct {
	store   *Store
	jobs    *jobs.Store
	artwork *artwork.Service
}

func NewAdminHandlers(st *Store, jb *jobs.Store, art *artwork.Service) *AdminHandlers {
	return &AdminHandlers{store: st, jobs: jb, artwork: art}
}

// Translation is one language's hand-edited or TMDB-fetched text.
type Translation struct {
	Name     string `json:"name"`
	Overview string `json:"overview"`
}

// decodeTranslations reads a translations jsonb column (language code ->
// Translation) for the editors.
func decodeTranslations(raw json.RawMessage) (map[string]Translation, error) {
	out := map[string]Translation{}
	if len(raw) == 0 {
		return out, nil
	}
	return out, json.Unmarshal(raw, &out)
}

type idInput struct {
	ID string `path:"id" format:"uuid"`
}

type idLangInput struct {
	ID   string `path:"id" format:"uuid"`
	Lang string `path:"lang" minLength:"2" maxLength:"5"`
}

type libraryInput struct {
	Kind     string `query:"type" enum:"movie,series"`
	Status   string `query:"status" enum:"draft,processing,published,hidden"`
	Query    string `query:"q"`
	Sort     string `query:"sort" enum:"added,name,year,size"`
	Page     int    `query:"page" minimum:"1" default:"1"`
	PageSize int    `query:"pageSize" minimum:"1" maximum:"200" default:"50"`
}

type LibraryPage struct {
	Items []LibraryRow `json:"items"`
	Total int          `json:"total"`
	Page  int          `json:"page"`
}

type libraryOutput struct{ Body LibraryPage }

func (h *AdminHandlers) Library(ctx context.Context, in *libraryInput) (*libraryOutput, error) {
	f := LibraryFilter{
		Kind:     in.Kind,
		Status:   in.Status,
		Query:    in.Query,
		Sort:     in.Sort,
		Page:     in.Page,
		PageSize: in.PageSize,
	}
	items, total, err := h.store.ListLibrary(ctx, f)
	if err != nil {
		return nil, err
	}
	return &libraryOutput{Body: LibraryPage{Items: items, Total: total, Page: f.Page}}, nil
}

type createTitleInput struct{ Body TitleInput }

type titleCreatedOutput struct {
	Status int
	Body   *Title
}

func (h *AdminHandlers) Create(ctx context.Context, in *createTitleInput) (*titleCreatedOutput, error) {
	t, err := h.store.CreateTitle(ctx, in.Body)
	if err != nil {
		return nil, err
	}
	return &titleCreatedOutput{Status: http.StatusCreated, Body: t}, nil
}

// AdminTitle is everything the title editor needs in one read. Translations
// are keyed by language code; Seasons is empty for movies. SubtitlesByFile is keyed by
// media file id.
type AdminTitle struct {
	Title           *Title                      `json:"title"`
	Translations    map[string]Translation      `json:"translations"`
	Seasons         []Season                    `json:"seasons"`
	MediaFiles      []media.MediaFile           `json:"mediaFiles"`
	SubtitlesByFile map[string][]media.Subtitle `json:"subtitlesByFile"`
	Artwork         []artwork.Artwork           `json:"artwork"`
}

type adminTitleOutput struct{ Body AdminTitle }

func (h *AdminHandlers) Get(ctx context.Context, in *idInput) (*adminTitleOutput, error) {
	t, err := h.store.TitleByID(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	out := AdminTitle{Title: t, Seasons: []Season{}}
	if out.Translations, err = decodeTranslations(t.Translations); err != nil {
		return nil, err
	}
	if t.Kind == "series" {
		if out.Seasons, err = h.store.SeasonsWithEpisodes(ctx, t.ID); err != nil {
			return nil, err
		}
	}
	if out.MediaFiles, err = h.store.MediaFilesForTitle(ctx, t.ID); err != nil {
		return nil, err
	}

	fileIDs := make([]string, len(out.MediaFiles))
	for i, f := range out.MediaFiles {
		fileIDs[i] = f.ID
	}
	if out.SubtitlesByFile, err = h.store.SubtitlesForMediaFiles(ctx, fileIDs); err != nil {
		return nil, err
	}
	if out.Artwork, err = h.artwork.Store.ArtworkFor(ctx, "title", t.ID); err != nil {
		return nil, err
	}
	return &adminTitleOutput{Body: out}, nil
}

type updateTitleInput struct {
	ID   string `path:"id" format:"uuid"`
	Body TitleUpdate
}

type titleOutput struct{ Body *Title }

func (h *AdminHandlers) Update(ctx context.Context, in *updateTitleInput) (*titleOutput, error) {
	t, err := h.store.UpdateTitle(ctx, in.ID, in.Body)
	if err != nil {
		return nil, err
	}
	return &titleOutput{Body: t}, nil
}

type setTranslationInput struct {
	ID   string `path:"id" format:"uuid"`
	Lang string `path:"lang" minLength:"2" maxLength:"5"`
	Body Translation
}

// SetTranslation saves a manually-edited name/overview for one language, so
// admins can localize titles that were not fetched from TMDB.
func (h *AdminHandlers) SetTranslation(ctx context.Context, in *setTranslationInput) (*struct{}, error) {
	return nil, h.store.SetTitleTranslationText(ctx, in.ID, in.Lang, in.Body.Name, in.Body.Overview)
}

type storageOutput struct{ Body *TitleStorageBreakdown }

// Storage returns a per-title disk-usage breakdown for the editor chart: one
// entry per episode (series) or per media file (movie), split into source and
// transcoded bytes, plus totals.
func (h *AdminHandlers) Storage(ctx context.Context, in *idInput) (*storageOutput, error) {
	t, err := h.store.TitleByID(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	breakdown, err := h.store.TitleStorage(ctx, in.ID, t.Kind)
	if err != nil {
		return nil, err
	}
	return &storageOutput{Body: breakdown}, nil
}

// DeleteLanguage removes one content language from a title: its translations
// across the title and all seasons/episodes, plus the code itself (promoting the
// next language to base when the removed one was the base). The matching audio
// files and subtitles are deleted by the client through their own endpoints;
// this owns only the catalog (title/season/episode) side.
func (h *AdminHandlers) DeleteLanguage(ctx context.Context, in *idLangInput) (*struct{}, error) {
	if err := h.store.RemoveContentLanguage(ctx, in.ID, in.Lang); err != nil {
		if errors.Is(err, ErrLastLanguage) {
			return nil, httpx.BadRequestError(err.Error())
		}
		return nil, err
	}
	return nil, nil
}

func (h *AdminHandlers) Delete(ctx context.Context, in *idInput) (*struct{}, error) {
	if err := h.store.DeleteTitle(ctx, in.ID); err != nil {
		return nil, err
	}
	return nil, h.artwork.DeleteForOwner(ctx, "title", in.ID)
}

type bulkInput struct {
	Body struct {
		IDs    []string `json:"ids" minItems:"1"`
		Action string   `json:"action" enum:"publish,hide,draft,delete,rescan"`
	}
}

func (h *AdminHandlers) Bulk(ctx context.Context, in *bulkInput) (*struct{}, error) {
	ids := in.Body.IDs
	switch in.Body.Action {
	case "publish":
		return nil, h.store.SetTitlesStatus(ctx, ids, "published")
	case "hide":
		return nil, h.store.SetTitlesStatus(ctx, ids, "hidden")
	case "draft":
		return nil, h.store.SetTitlesStatus(ctx, ids, "draft")
	case "delete":
		if err := h.store.DeleteTitles(ctx, ids); err != nil {
			return nil, err
		}
		for _, id := range ids {
			if err := h.artwork.DeleteForOwner(ctx, "title", id); err != nil {
				return nil, err
			}
		}
	case "rescan":
		fileIDs, err := h.store.MediaFileIDsForTitles(ctx, ids)
		if err != nil {
			return nil, err
		}
		for _, id := range fileIDs {
			if _, err := h.jobs.EnqueueJobOnce(ctx, "probe",
				map[string]string{"mediaFileId": id}, jobs.EnqueueOpts{}); err != nil {
				return nil, err
			}
		}
	}
	return nil, nil
}

type createSeasonInput struct {
	ID   string `path:"id" format:"uuid"`
	Body struct {
		SeasonNumber int    `json:"seasonNumber" minimum:"0"`
		Name         string `json:"name" required:"false"`
	}
}

type seasonCreatedOutput struct {
	Status int
	Body   *Season
}

func (h *AdminHandlers) CreateSeason(ctx context.Context, in *createSeasonInput) (*seasonCreatedOutput, error) {
	se, err := h.store.CreateSeason(ctx, in.ID, in.Body.SeasonNumber, in.Body.Name)
	if err != nil {
		return nil, err
	}
	return &seasonCreatedOutput{Status: http.StatusCreated, Body: se}, nil
}

func (h *AdminHandlers) DeleteSeason(ctx context.Context, in *idInput) (*struct{}, error) {
	return nil, h.store.DeleteSeason(ctx, in.ID)
}

type createEpisodeInput struct {
	ID   string `path:"id" format:"uuid"`
	Body EpisodeInput
}

type episodeCreatedOutput struct {
	Status int
	Body   *Episode
}

func (h *AdminHandlers) CreateEpisode(ctx context.Context, in *createEpisodeInput) (*episodeCreatedOutput, error) {
	e, err := h.store.CreateEpisode(ctx, in.ID, in.Body)
	if err != nil {
		return nil, err
	}
	return &episodeCreatedOutput{Status: http.StatusCreated, Body: e}, nil
}

type updateEpisodeInput struct {
	ID   string `path:"id" format:"uuid"`
	Body EpisodeUpdate
}

type episodeOutput struct{ Body *Episode }

func (h *AdminHandlers) UpdateEpisode(ctx context.Context, in *updateEpisodeInput) (*episodeOutput, error) {
	e, err := h.store.UpdateEpisode(ctx, in.ID, in.Body)
	if err != nil {
		return nil, err
	}
	return &episodeOutput{Body: e}, nil
}

type translationsOutput struct {
	// Body is keyed by language code.
	Body map[string]Translation
}

// EpisodeTranslations returns an episode's translations for the editor.
func (h *AdminHandlers) EpisodeTranslations(ctx context.Context, in *idInput) (*translationsOutput, error) {
	raw, err := h.store.EpisodeTranslations(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	tr, err := decodeTranslations(raw)
	if err != nil {
		return nil, err
	}
	return &translationsOutput{Body: tr}, nil
}

// SetEpisodeTranslation saves a manually-edited name/overview for one language.
func (h *AdminHandlers) SetEpisodeTranslation(ctx context.Context, in *setTranslationInput) (*struct{}, error) {
	return nil, h.store.SetEpisodeTranslation(ctx, in.ID, in.Lang, in.Body.Name, in.Body.Overview)
}

func (h *AdminHandlers) DeleteEpisode(ctx context.Context, in *idInput) (*struct{}, error) {
	return nil, h.store.DeleteEpisode(ctx, in.ID)
}
