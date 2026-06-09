package api

import (
	"net/http"
	"strconv"

	"couchverse/internal/artwork"
	"couchverse/internal/httpx"
	"couchverse/internal/store"
)

type Artwork struct {
	store   *store.Store
	service *artwork.Service
}

func NewArtwork(st *store.Store, service *artwork.Service) *Artwork {
	return &Artwork{store: st, service: service}
}

// Serve returns the artwork image, resized on first request when ?size= is given.
func (h *Artwork) Serve(w http.ResponseWriter, r *http.Request) {
	art, err := h.store.ArtworkByID(r.Context(), httpx.ID(r, "id"))
	if err != nil {
		respondStoreErr(w, err)
		return
	}
	path, err := h.service.Resolve(r.Context(), art, r.URL.Query().Get("size"))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=86400")
	http.ServeFile(w, r, path)
}

var artworkOwnerKinds = map[string]bool{
	"title": true, "season": true, "episode": true, "artist": true, "album": true,
}
var artworkKinds = map[string]bool{
	"poster": true, "backdrop": true, "thumb": true, "album_cover": true, "artist_photo": true,
}

// Upload accepts multipart form data: ownerKind, ownerId, kind, file.
func (h *Artwork) Upload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		httpx.BadRequest(w, "invalid multipart form")
		return
	}
	ownerKind := r.FormValue("ownerKind")
	kind := r.FormValue("kind")
	ownerID, _ := strconv.ParseInt(r.FormValue("ownerId"), 10, 64)
	if !artworkOwnerKinds[ownerKind] || !artworkKinds[kind] {
		httpx.BadRequest(w, "invalid ownerKind or kind")
		return
	}
	if ownerID <= 0 {
		httpx.BadRequest(w, "ownerId is required")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		httpx.BadRequest(w, "file field is required")
		return
	}
	defer file.Close()

	art, err := h.service.Save(r.Context(), ownerKind, ownerID, kind, header.Filename, file)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "artwork_failed", err.Error())
		return
	}
	httpx.JSON(w, http.StatusCreated, art)
}

func (h *Artwork) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.service.Delete(r.Context(), httpx.ID(r, "id")); err != nil {
		respondStoreErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}
