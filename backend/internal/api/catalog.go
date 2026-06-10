package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"couchverse/internal/feature/artwork"
	"couchverse/internal/feature/auth"
	"couchverse/internal/feature/music"
	"couchverse/internal/flags"
	"couchverse/internal/httpx"
	"couchverse/internal/settings"
	"couchverse/internal/store"
)

type Catalog struct {
	store    *store.Store
	settings *settings.Store
	artwork  *artwork.Store
	music    *music.Store
}

func NewCatalog(st *store.Store, set *settings.Store, art *artwork.Store, mus *music.Store) *Catalog {
	return &Catalog{store: st, settings: set, artwork: art, music: mus}
}

func (h *Catalog) Home(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r.Context())

	featured, err := h.store.FeaturedTitle(r.Context())
	if err != nil && !errors.Is(err, httpx.ErrNotFound) {
		httpx.Internal(w, err)
		return
	}

	var featuredBackdropID *string
	featuredInList := false
	if featured != nil {
		art, aerr := h.artwork.ArtworkFor(r.Context(), "title", featured.ID)
		if aerr != nil {
			httpx.Internal(w, aerr)
			return
		}
		for _, a := range art {
			if a.Kind == "backdrop" {
				id := a.ID
				featuredBackdropID = &id
			}
		}
		if featuredInList, err = h.store.WatchlistHas(r.Context(), user.ID, featured.ID); err != nil {
			httpx.Internal(w, err)
			return
		}
	}

	configs, err := h.store.HomeRowConfigs(r.Context())
	if err != nil {
		httpx.Internal(w, err)
		return
	}

	flags := flags.Load(r.Context(), h.settings)
	rows := []store.HomeRow{}
	for _, cfg := range configs {
		var items any
		switch cfg.Kind {
		case "continue_watching":
			items, err = h.store.ContinueWatching(r.Context(), user.ID, 20)
		case "recently_added":
			items, err = h.store.RecentlyAdded(r.Context(), 20)
		case "genre":
			if cfg.GenreID == nil {
				continue
			}
			items, err = h.store.TitlesByGenre(r.Context(), *cfg.GenreID, 20)
		case "recently_played_music":
			if !flags.MusicEnabled {
				continue
			}
			items, err = h.music.RecentlyPlayedAlbums(r.Context(), user.ID, 20)
		default:
			continue
		}
		if err != nil {
			httpx.Internal(w, err)
			return
		}
		rows = append(rows, store.HomeRow{Kind: cfg.Kind, Label: cfg.Label, Items: items})
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"featured":           featured,
		"featuredBackdropId": featuredBackdropID,
		"featuredInList":     featuredInList,
		"rows":               rows,
	})
}

func (h *Catalog) Browse(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	items, total, err := h.store.BrowseTitles(r.Context(), store.BrowseFilter{
		Kind:  q.Get("kind"),
		Genre: q.Get("genre"),
		Query: q.Get("q"),
		Sort:  q.Get("sort"),
		Page:  httpx.QueryInt(r, "page", 1),
	})
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}

func (h *Catalog) Title(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r.Context())
	slug := chi.URLParam(r, "slug")
	if slug == "" {
		httpx.NotFound(w)
		return
	}
	t, err := h.store.TitleBySlug(r.Context(), slug)
	if err != nil {
		httpx.StoreErr(w, err)
		return
	}
	if t.Status != "published" {
		httpx.NotFound(w)
		return
	}

	out := map[string]any{"title": t}

	watchlisted, err := h.store.WatchlistHas(r.Context(), user.ID, t.ID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	out["inWatchlist"] = watchlisted

	files, err := h.store.MediaFilesForTitle(r.Context(), t.ID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	out["mediaFiles"] = files

	artwork, err := h.artwork.ArtworkFor(r.Context(), "title", t.ID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	out["artwork"] = artwork

	if t.Kind == "series" {
		seasons, err := h.store.SeasonsWithEpisodes(r.Context(), t.ID)
		if err != nil {
			httpx.Internal(w, err)
			return
		}
		out["seasons"] = seasons

		progress, err := h.store.EpisodeProgressForTitle(r.Context(), user.ID, t.ID)
		if err != nil {
			httpx.Internal(w, err)
			return
		}
		out["episodeProgress"] = progress
	} else {
		position, duration, err := h.store.ProgressFor(r.Context(), user.ID, &t.ID, nil)
		if err != nil {
			httpx.Internal(w, err)
			return
		}
		out["progress"] = store.EpisodeProgress{Position: position, Duration: duration}
	}

	httpx.JSON(w, http.StatusOK, out)
}

func (h *Catalog) Search(w http.ResponseWriter, r *http.Request) {
	includeMusic := flags.Load(r.Context(), h.settings).MusicEnabled
	res, err := h.store.Search(r.Context(), r.URL.Query().Get("q"), 12, includeMusic)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}
