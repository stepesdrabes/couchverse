package api

import (
	"net/http"

	"couchverse/internal/auth"
	"couchverse/internal/httpx"
	"couchverse/internal/store"
)

type Music struct {
	store *store.Store
}

func NewMusic(st *store.Store) *Music {
	return &Music{store: st}
}

func (h *Music) Home(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r.Context())

	albums, err := h.store.RecentAlbums(r.Context(), 24)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	artists, err := h.store.ListArtists(r.Context())
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	playlists, err := h.store.ListPlaylists(r.Context(), user.ID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	recent, err := h.store.RecentlyPlayedAlbums(r.Context(), user.ID, 12)
	if err != nil {
		httpx.Internal(w, err)
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"recentAlbums":   albums,
		"artists":        artists,
		"playlists":      playlists,
		"recentlyPlayed": recent,
	})
}

func (h *Music) Album(w http.ResponseWriter, r *http.Request) {
	id := httpx.UUID(r, "id")
	if id == "" {
		httpx.NotFound(w)
		return
	}
	album, err := h.store.AlbumCardByID(r.Context(), id)
	if err != nil {
		httpx.StoreErr(w, err)
		return
	}
	tracks, err := h.store.TracksForAlbum(r.Context(), album.ID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"album": album, "tracks": tracks})
}

func (h *Music) Artist(w http.ResponseWriter, r *http.Request) {
	id := httpx.UUID(r, "id")
	if id == "" {
		httpx.NotFound(w)
		return
	}
	artist, err := h.store.ArtistByID(r.Context(), id)
	if err != nil {
		httpx.StoreErr(w, err)
		return
	}
	albums, err := h.store.AlbumsByArtist(r.Context(), artist.ID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"artist": artist, "albums": albums})
}

// Scrobble records a track play for "recently played".
func (h *Music) Scrobble(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TrackID string `json:"trackId"`
	}
	if err := httpx.Decode(r, &req); err != nil || req.TrackID == "" {
		httpx.BadRequest(w, "trackId is required")
		return
	}
	if err := h.store.RecordPlay(r.Context(), auth.UserFrom(r.Context()).ID, req.TrackID); err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}
