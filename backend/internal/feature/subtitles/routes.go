package subtitles

import (
	"github.com/go-chi/chi/v5"
)

func (h *Subtitles) MountUser(r chi.Router) {
	r.Get("/subtitles/{id}.vtt", h.Serve)
}

func (h *Subtitles) MountAdmin(r chi.Router) {
	r.Get("/media-files/{id}/subtitles", h.ListForMediaFile)
	r.Post("/media-files/{id}/subtitles", h.Upload)
	r.Delete("/subtitles/{id}", h.Delete)
}
