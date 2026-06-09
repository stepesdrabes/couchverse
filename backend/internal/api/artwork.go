package api

import (
	"net/http"
	"path/filepath"

	"couchverse/internal/httpx"
	"couchverse/internal/store"
)

type Artwork struct {
	store   *store.Store
	dataDir string
}

func NewArtwork(st *store.Store, dataDir string) *Artwork {
	return &Artwork{store: st, dataDir: dataDir}
}

// Serve returns the artwork image. Sized variants arrive with the artwork
// pipeline milestone; for now the original is served with long-lived caching.
func (h *Artwork) Serve(w http.ResponseWriter, r *http.Request) {
	art, err := h.store.ArtworkByID(r.Context(), httpx.ID(r, "id"))
	if err != nil {
		respondStoreErr(w, err)
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=86400")
	http.ServeFile(w, r, filepath.Join(h.dataDir, art.Path))
}
