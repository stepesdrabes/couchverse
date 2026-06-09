package api

import (
	"net/http"

	"couchverse/internal/httpx"
	"couchverse/internal/store"
	"couchverse/internal/subtitles"
)

type Subtitles struct {
	store   *store.Store
	service *subtitles.Service
}

func NewSubtitles(st *store.Store, service *subtitles.Service) *Subtitles {
	return &Subtitles{store: st, service: service}
}

// Serve returns the WebVTT file for a subtitle track.
func (h *Subtitles) Serve(w http.ResponseWriter, r *http.Request) {
	sub, err := h.store.SubtitleByID(r.Context(), httpx.ID(r, "id"))
	if err != nil {
		respondStoreErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	w.Header().Set("Cache-Control", "private, max-age=86400")
	http.ServeFile(w, r, h.service.Path(sub))
}

// Upload accepts multipart form data: lang, label?, file (.srt/.vtt).
func (h *Subtitles) Upload(w http.ResponseWriter, r *http.Request) {
	mediaFileID := httpx.ID(r, "id")
	if _, err := h.store.MediaFileByID(r.Context(), mediaFileID); err != nil {
		respondStoreErr(w, err)
		return
	}
	if err := r.ParseMultipartForm(16 << 20); err != nil {
		httpx.BadRequest(w, "invalid multipart form")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		httpx.BadRequest(w, "file field is required")
		return
	}
	defer file.Close()

	sub, err := h.service.SaveUpload(r.Context(), mediaFileID,
		r.FormValue("lang"), r.FormValue("label"), header.Filename, file)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "subtitle_failed", err.Error())
		return
	}
	httpx.JSON(w, http.StatusCreated, sub)
}

func (h *Subtitles) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.service.Delete(r.Context(), httpx.ID(r, "id")); err != nil {
		respondStoreErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}

func (h *Subtitles) ListForMediaFile(w http.ResponseWriter, r *http.Request) {
	subs, err := h.store.SubtitlesForMediaFile(r.Context(), httpx.ID(r, "id"))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, subs)
}
