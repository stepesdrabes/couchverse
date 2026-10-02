package subtitles

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"couchverse/internal/httpx"
)

func (h *Subtitles) Register(rt httpx.Routes) {
	tags := []string{"subtitles"}
	httpx.Raw(rt.User, huma.Operation{OperationID: "getSubtitle", Method: http.MethodGet, Path: "/subtitles/{id}.vtt", Tags: tags}, h.Serve)

	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminListSubtitles", Method: http.MethodGet, Path: "/media-files/{id}/subtitles", Tags: tags}, h.ListForMediaFile)
	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminUploadSubtitle", Method: http.MethodPost, Path: "/media-files/{id}/subtitles", Tags: tags}, h.Upload)
	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminDeleteSubtitle", Method: http.MethodDelete, Path: "/subtitles/{id}", Tags: tags}, h.Delete)
}
