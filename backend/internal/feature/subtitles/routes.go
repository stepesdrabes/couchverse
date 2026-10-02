package subtitles

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"couchverse/internal/httpx"
)

const tag httpx.Tag = "subtitles"

func (h *Subtitles) Register(rt httpx.Routes) {
	httpx.Raw(rt.User, serveOp(rt.User), h.Serve)

	huma.Register(rt.Admin, tag.Op("adminListSubtitles", http.MethodGet, "/media-files/{id}/subtitles"), h.ListForMediaFile)
	huma.Register(rt.Admin, tag.Created("adminUploadSubtitle", http.MethodPost, "/media-files/{id}/subtitles"), h.Upload)
	huma.Register(rt.Admin, tag.NoContent("adminDeleteSubtitle", http.MethodDelete, "/subtitles/{id}"), h.Delete)
}

// serveOp documents the WebVTT route, which stays a plain handler because it
// answers with the file itself rather than JSON.
func serveOp(api huma.API) huma.Operation {
	op := tag.Op("getSubtitle", http.MethodGet, "/subtitles/{id}.vtt")
	op.Summary = "Get a subtitle track"
	op.Description = "The track as a WebVTT side-car file for the player."
	op.Parameters = []*huma.Param{
		{Name: "id", In: "path", Required: true, Schema: &huma.Schema{Type: huma.TypeString, Format: "uuid"}},
	}
	op.Responses = map[string]*huma.Response{
		"200": {Description: "The WebVTT file", Content: map[string]*huma.MediaType{
			"text/vtt": {Schema: &huma.Schema{Type: huma.TypeString}},
		}},
		"304":     {Description: "Not modified since If-Modified-Since"},
		"default": httpx.ErrorResponse(api),
	}
	return op
}
