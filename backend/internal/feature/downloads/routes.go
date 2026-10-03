package downloads

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"couchverse/internal/httpx"
)

const tag httpx.Tag = "downloads"

func (h *Handlers) Register(rt httpx.Routes) {
	request := tag.Op("requestDownload", http.MethodPost, "/me/downloads")
	request.Summary = "Ask for a movie or an episode to be prepared for offline viewing"
	request.Description = "Plans a device-ready MP4 (faststart; the source video when the profile decodes it, else H.264 at the quality; " +
		"the audio languages asked for, copied when the device takes them and AAC otherwise; text subtitles as mov_text) " +
		"and queues its preparation, or reuses the MP4 another request already made. Asking again returns the same download " +
		"and prepares a failed one again. Poll getDownload until it is ready, then fetch url. " +
		"Codes: no_media (404), source_deleted (409), unsupported (422, the device plays no download of this file)."
	huma.Register(rt.User, httpx.Localized(request), h.Request)
	huma.Register(rt.User, httpx.Localized(tag.Op("listDownloads", http.MethodGet, "/me/downloads")), h.List)
	huma.Register(rt.User, httpx.Localized(tag.Op("getDownload", http.MethodGet, "/me/downloads/{id}")), h.Get)
	remove := tag.NoContent("deleteDownload", http.MethodDelete, "/me/downloads/{id}")
	remove.Description = "Removes the download from the list; a device that fetched the MP4 keeps its copy."
	huma.Register(rt.User, remove, h.Delete)

	httpx.Raw(rt.Media, fetchOp(httpx.ErrorResponse(rt.Media)), h.Serve)
}

func fetchOp(apiErr *huma.Response) huma.Operation {
	op := tag.Op("fetchDownload", http.MethodGet, "/downloads/{id}")
	op.Summary = "Fetch a prepared download"
	op.Description = "Serves the MP4 with HTTP range support, so an interrupted transfer resumes. Use the url of a ready " +
		"download. 404 codes: not_found (no such download of the granted file), file_missing; 409 not_ready."
	op.Parameters = []*huma.Param{
		{Name: "id", In: "path", Required: true, Description: "The prepared file id from the download's url.",
			Schema: &huma.Schema{Type: huma.TypeString, Format: "uuid"}},
		{Name: "Range", In: "header", Description: "Byte range to read, e.g. bytes=0-1048575.", Schema: &huma.Schema{Type: huma.TypeString}},
	}
	binary := &huma.Schema{Type: huma.TypeString, Format: "binary"}
	op.Responses = map[string]*huma.Response{
		"200":     {Description: "The whole file.", Content: map[string]*huma.MediaType{"video/mp4": {Schema: binary}}},
		"206":     {Description: "The requested byte range.", Content: map[string]*huma.MediaType{"video/mp4": {Schema: binary}}},
		"416":     {Description: "The range lies outside the file."},
		"default": apiErr,
	}
	return op
}
