package metadata

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"couchverse/internal/httpx"
)

const tag httpx.Tag = "metadata"

func (h *AdminMetadata) Register(rt httpx.Routes) {
	huma.Register(rt.Admin, tag.Op("adminSearchMetadata", http.MethodGet, "/metadata/search"), h.Search)
	huma.Register(rt.Admin, accepted(tag.Op("adminApplyMetadata", http.MethodPost, "/titles/{id}/metadata/apply")), h.Apply)
	huma.Register(rt.Admin, tag.Op("adminListMetadataSeasons", http.MethodGet, "/titles/{id}/metadata/seasons"), h.Seasons)
	huma.Register(rt.Admin, accepted(tag.Op("adminImportEpisodes", http.MethodPost, "/titles/{id}/metadata/import-episodes")), h.ImportEpisodes)
}

// accepted marks an operation that answers 202: the work runs as a queued job.
func accepted(op huma.Operation) huma.Operation {
	op.DefaultStatus = http.StatusAccepted
	return op
}
