package api

import (
	"net/http"

	"couchverse/internal/auth"
	"couchverse/internal/httpx"
	"couchverse/internal/store"
)

type Playlists struct {
	store *store.Store
}

func NewPlaylists(st *store.Store) *Playlists {
	return &Playlists{store: st}
}

func (h *Playlists) List(w http.ResponseWriter, r *http.Request) {
	playlists, err := h.store.ListPlaylists(r.Context(), auth.UserFrom(r.Context()).ID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, playlists)
}

func (h *Playlists) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := httpx.Decode(r, &req); err != nil || req.Name == "" {
		httpx.BadRequest(w, "name is required")
		return
	}
	playlist, err := h.store.CreatePlaylist(r.Context(), auth.UserFrom(r.Context()).ID, req.Name)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, playlist)
}

func (h *Playlists) Get(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r.Context())
	playlist, err := h.store.PlaylistForUser(r.Context(), user.ID, httpx.ID(r, "id"))
	if err != nil {
		respondStoreErr(w, err)
		return
	}
	entries, err := h.store.PlaylistEntries(r.Context(), playlist.ID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"playlist": playlist, "entries": entries})
}

func (h *Playlists) Rename(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := httpx.Decode(r, &req); err != nil || req.Name == "" {
		httpx.BadRequest(w, "name is required")
		return
	}
	if err := h.store.RenamePlaylist(r.Context(), auth.UserFrom(r.Context()).ID, httpx.ID(r, "id"), req.Name); err != nil {
		respondStoreErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}

func (h *Playlists) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.store.DeletePlaylist(r.Context(), auth.UserFrom(r.Context()).ID, httpx.ID(r, "id")); err != nil {
		respondStoreErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}

// requirePlaylist loads the playlist ensuring ownership.
func (h *Playlists) requirePlaylist(w http.ResponseWriter, r *http.Request) *store.Playlist {
	playlist, err := h.store.PlaylistForUser(r.Context(), auth.UserFrom(r.Context()).ID, httpx.ID(r, "id"))
	if err != nil {
		respondStoreErr(w, err)
		return nil
	}
	return playlist
}

func (h *Playlists) AddTrack(w http.ResponseWriter, r *http.Request) {
	playlist := h.requirePlaylist(w, r)
	if playlist == nil {
		return
	}
	var req struct {
		TrackID int64 `json:"trackId"`
	}
	if err := httpx.Decode(r, &req); err != nil || req.TrackID <= 0 {
		httpx.BadRequest(w, "trackId is required")
		return
	}
	if err := h.store.AddPlaylistTrack(r.Context(), playlist.ID, req.TrackID); err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}

func (h *Playlists) RemoveEntry(w http.ResponseWriter, r *http.Request) {
	playlist := h.requirePlaylist(w, r)
	if playlist == nil {
		return
	}
	if err := h.store.RemovePlaylistEntry(r.Context(), playlist.ID, httpx.ID(r, "entryId")); err != nil {
		respondStoreErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}

func (h *Playlists) Reorder(w http.ResponseWriter, r *http.Request) {
	playlist := h.requirePlaylist(w, r)
	if playlist == nil {
		return
	}
	var req struct {
		EntryIDs []int64 `json:"entryIds"`
	}
	if err := httpx.Decode(r, &req); err != nil || len(req.EntryIDs) == 0 {
		httpx.BadRequest(w, "entryIds are required")
		return
	}
	if err := h.store.ReorderPlaylist(r.Context(), playlist.ID, req.EntryIDs); err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}
