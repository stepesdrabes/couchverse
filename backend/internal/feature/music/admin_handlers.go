package music

import (
	"net/http"

	"couchverse/internal/feature/artwork"
	"couchverse/internal/httpx"
)

type AdminHandlers struct {
	store   *Store
	artwork *artwork.Service
}

func NewAdminHandlers(st *Store, art *artwork.Service) *AdminHandlers {
	return &AdminHandlers{store: st, artwork: art}
}

func (h *AdminHandlers) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	items, total, err := h.store.AdminListAlbums(r.Context(),
		q.Get("q"), q.Get("sort"), httpx.QueryInt(r, "page", 1), httpx.QueryInt(r, "pageSize", 50))
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}

func (h *AdminHandlers) Get(w http.ResponseWriter, r *http.Request) {
	id := httpx.UUID(r, "id")
	if id == "" {
		httpx.NotFound(w)
		return
	}
	album, err := h.store.AdminAlbumByID(r.Context(), id)
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

func (h *AdminHandlers) Update(w http.ResponseWriter, r *http.Request) {
	id := httpx.UUID(r, "id")
	if id == "" {
		httpx.NotFound(w)
		return
	}
	var up AlbumUpdate
	if err := httpx.Decode(r, &up); err != nil {
		httpx.BadRequest(w, "invalid request body")
		return
	}
	if up.Status != nil && !validStatus(*up.Status) {
		httpx.BadRequest(w, "invalid status")
		return
	}
	if err := h.store.UpdateAlbum(r.Context(), id, up); err != nil {
		httpx.StoreErr(w, err)
		return
	}
	h.Get(w, r)
}

func (h *AdminHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	id := httpx.UUID(r, "id")
	if id == "" {
		httpx.NotFound(w)
		return
	}
	if err := h.store.DeleteAlbum(r.Context(), id); err != nil {
		httpx.StoreErr(w, err)
		return
	}
	if err := h.artwork.DeleteForOwner(r.Context(), "album", id); err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}

func (h *AdminHandlers) UpdateTrack(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := httpx.Decode(r, &req); err != nil || req.Name == "" {
		httpx.BadRequest(w, "name is required")
		return
	}
	id := httpx.UUID(r, "id")
	if id == "" {
		httpx.NotFound(w)
		return
	}
	if err := h.store.UpdateTrackName(r.Context(), id, req.Name); err != nil {
		httpx.StoreErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}

func (h *AdminHandlers) DeleteTrack(w http.ResponseWriter, r *http.Request) {
	id := httpx.UUID(r, "id")
	if id == "" {
		httpx.NotFound(w)
		return
	}
	if err := h.store.DeleteTrack(r.Context(), id); err != nil {
		httpx.StoreErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}

func validStatus(s string) bool {
	switch s {
	case "draft", "processing", "published", "hidden":
		return true
	}
	return false
}
