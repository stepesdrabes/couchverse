package library

import (
	"github.com/go-chi/chi/v5"
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

func (m *Module) MountAdmin(r chi.Router) {
	r.Patch("/media-files/{id}", m.libraries.SetMediaFileAudio)
	r.Delete("/media-files/{id}", m.libraries.DeleteMediaFile)

	r.Get("/uploads", m.uploads.List)
	r.Post("/uploads", m.uploads.Create)
	r.Get("/uploads/{id}", m.uploads.Get)
	r.Put("/uploads/{id}", m.uploads.Append)
	r.Post("/uploads/{id}/complete", m.uploads.Complete)
	r.Delete("/uploads/{id}", m.uploads.Abort)
}
