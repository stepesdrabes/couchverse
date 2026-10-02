package library

import (
	"net/http"
	"reflect"

	"github.com/danielgtaylor/huma/v2"

	"couchverse/internal/httpx"
)

// Module bundles the library and upload admin handlers.
type Module struct {
	libraries *AdminLibraries
	uploads   *AdminUploads
}

func NewModule(st *Store, manager *Manager) *Module {
	return &Module{
		libraries: NewAdminLibraries(st, manager.DataDir),
		uploads:   NewAdminUploads(st, manager),
	}
}

const tag httpx.Tag = "library"

func (m *Module) Register(rt httpx.Routes) {
	huma.Register(rt.Admin, tag.NoContent("adminUpdateMediaFile", http.MethodPatch, "/media-files/{id}"), m.libraries.SetMediaFileAudio)
	huma.Register(rt.Admin, tag.NoContent("adminDeleteMediaFile", http.MethodDelete, "/media-files/{id}"), m.libraries.DeleteMediaFile)

	huma.Register(rt.Admin, tag.Op("adminListUploads", http.MethodGet, "/uploads"), m.uploads.List)
	create := tag.Op("adminCreateUpload", http.MethodPost, "/uploads")
	create.DefaultStatus = http.StatusCreated
	huma.Register(rt.Admin, create, m.uploads.Create)
	huma.Register(rt.Admin, tag.Op("adminGetUpload", http.MethodGet, "/uploads/{id}"), m.uploads.Get)
	httpx.Raw(rt.Admin, appendUploadOp(rt.Admin), m.uploads.Append)
	huma.Register(rt.Admin, tag.Op("adminCompleteUpload", http.MethodPost, "/uploads/{id}/complete"), m.uploads.Complete)
	huma.Register(rt.Admin, tag.NoContent("adminAbortUpload", http.MethodDelete, "/uploads/{id}"), m.uploads.Abort)
}

// appendUploadOp describes the chunk PUT, which stays a plain handler because
// the body is the raw chunk rather than JSON.
func appendUploadOp(api huma.API) huma.Operation {
	schemas := api.OpenAPI().Components.Schemas
	jsonOf := func(desc string, t reflect.Type) *huma.Response {
		return &huma.Response{Description: desc, Content: map[string]*huma.MediaType{
			"application/json": {Schema: schemas.Schema(t, true, "")},
		}}
	}
	zero := 0.0
	op := tag.Op("adminAppendUpload", http.MethodPut, "/uploads/{id}")
	op.Summary = "Append a chunk to an upload"
	op.Description = "Chunks are sequential: offset must equal the bytes the server already holds, " +
		"otherwise 409 answers that offset so the client can resume from it. " +
		"400 codes: bad_request (missing offset), append_failed (session not active, chunk too large, " +
		"more bytes than declared)."
	op.Parameters = []*huma.Param{
		{Name: "id", In: "path", Required: true, Schema: &huma.Schema{Type: huma.TypeString, Format: "uuid"}},
		{Name: "offset", In: "query", Required: true, Description: "Byte offset this chunk starts at.",
			Schema: &huma.Schema{Type: huma.TypeInteger, Format: "int64", Minimum: &zero}},
	}
	op.RequestBody = &huma.RequestBody{
		Description: "The chunk, at most 64 MiB.",
		Required:    true,
		Content: map[string]*huma.MediaType{
			"application/octet-stream": {Schema: &huma.Schema{Type: huma.TypeString, Format: "binary"}},
		},
	}
	offset := reflect.TypeFor[UploadOffset]()
	op.Responses = map[string]*huma.Response{
		"200":     jsonOf("The new offset after the chunk.", offset),
		"409":     jsonOf("The offset did not match; resume from the returned one.", offset),
		"default": jsonOf("Error", reflect.TypeFor[httpx.APIError]()),
	}
	return op
}
