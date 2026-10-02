package metadata

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"couchverse/internal/httpx"
)

func (h *AdminMetadata) Register(rt httpx.Routes) {
	tags := []string{"metadata"}
	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminSearchMetadata", Method: http.MethodGet, Path: "/metadata/search", Tags: tags}, h.Search)
	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminApplyMetadata", Method: http.MethodPost, Path: "/titles/{id}/metadata/apply", Tags: tags}, h.Apply)
	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminListMetadataSeasons", Method: http.MethodGet, Path: "/titles/{id}/metadata/seasons", Tags: tags}, h.Seasons)
	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminImportEpisodes", Method: http.MethodPost, Path: "/titles/{id}/metadata/import-episodes", Tags: tags}, h.ImportEpisodes)
}
