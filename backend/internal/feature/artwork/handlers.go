package artwork

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"couchverse/internal/httpx"
)

type Handlers struct {
	service *Service
}

func NewHandlers(service *Service) *Handlers {
	return &Handlers{service: service}
}

func (h *Handlers) MountUser(r chi.Router) {
	r.Get("/artwork/{id}", h.Serve)
}

func (h *Handlers) MountAdmin(r chi.Router) {
	r.Post("/artwork", h.Upload)
	r.Delete("/artwork/{id}", h.Delete)
}

// Serve returns the artwork image, resized on first request when ?size= is given.
func (h *Handlers) Serve(w http.ResponseWriter, r *http.Request) {
	id := httpx.UUID(r, "id")
	if id == "" {
		httpx.NotFound(w)
		return
	}
	art, err := h.service.Store.ArtworkByID(r.Context(), id)
	if err != nil {
		httpx.StoreErr(w, err)
		return
	}
	path, err := h.service.Resolve(r.Context(), art, r.URL.Query().Get("size"))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	// versioned URLs (?v=<token>) carry the artwork's updated time, so the bytes
	// for a given URL never change - cache them hard. Unversioned URLs keep
	// revalidating (ServeFile answers 304 via Last-Modified) so replaced artwork
	// shows up immediately even from a call site that doesn't pass a version.
	if r.URL.Query().Get("v") != "" {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "private, no-cache")
	}
	http.ServeFile(w, r, path)
}

var artworkOwnerKinds = map[string]bool{
	"title": true, "season": true, "episode": true, "artist": true, "album": true,
}
var artworkKinds = map[string]bool{
	"poster": true, "backdrop": true, "thumb": true, "album_cover": true, "artist_photo": true,
}

// Upload accepts multipart form data: ownerKind, ownerId, kind, file.
func (h *Handlers) Upload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		httpx.BadRequest(w, "invalid multipart form")
		return
	}
	ownerKind := r.FormValue("ownerKind")
	kind := r.FormValue("kind")
	ownerID := r.FormValue("ownerId")
	if !artworkOwnerKinds[ownerKind] || !artworkKinds[kind] {
		httpx.BadRequest(w, "invalid ownerKind or kind")
		return
	}
	if ownerID == "" {
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

func (h *Handlers) Delete(w http.ResponseWriter, r *http.Request) {
	id := httpx.UUID(r, "id")
	if id == "" {
		httpx.NotFound(w)
		return
	}
	if err := h.service.Delete(r.Context(), id); err != nil {
		httpx.StoreErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}
