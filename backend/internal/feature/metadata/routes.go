package metadata

import (
	"github.com/go-chi/chi/v5"
)

func (h *AdminMetadata) MountAdmin(r chi.Router) {
	r.Get("/metadata/search", h.Search)
	r.Post("/titles/{id}/metadata/apply", h.Apply)
	r.Get("/titles/{id}/metadata/seasons", h.Seasons)
	r.Post("/titles/{id}/metadata/import-episodes", h.ImportEpisodes)
}
