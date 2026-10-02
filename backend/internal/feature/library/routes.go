package library

import (
	"net/http"

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

func (m *Module) Register(rt httpx.Routes) {
	tags := []string{"library"}
	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminUpdateMediaFile", Method: http.MethodPatch, Path: "/media-files/{id}", Tags: tags}, m.libraries.SetMediaFileAudio)
	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminDeleteMediaFile", Method: http.MethodDelete, Path: "/media-files/{id}", Tags: tags}, m.libraries.DeleteMediaFile)

	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminListUploads", Method: http.MethodGet, Path: "/uploads", Tags: tags}, m.uploads.List)
	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminCreateUpload", Method: http.MethodPost, Path: "/uploads", Tags: tags}, m.uploads.Create)
	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminGetUpload", Method: http.MethodGet, Path: "/uploads/{id}", Tags: tags}, m.uploads.Get)
	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminAppendUpload", Method: http.MethodPut, Path: "/uploads/{id}", Tags: tags}, m.uploads.Append)
	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminCompleteUpload", Method: http.MethodPost, Path: "/uploads/{id}/complete", Tags: tags}, m.uploads.Complete)
	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminAbortUpload", Method: http.MethodDelete, Path: "/uploads/{id}", Tags: tags}, m.uploads.Abort)
}
