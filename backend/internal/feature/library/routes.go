package library

import (
	"github.com/go-chi/chi/v5"

	"couchverse/internal/feature/jobs"
)

// Module bundles the library and upload admin handlers.
type Module struct {
	libraries *AdminLibraries
	uploads   *AdminUploads
}

func NewModule(st *Store, jb *jobs.Store, manager *Manager) *Module {
	return &Module{
		libraries: NewAdminLibraries(st, jb),
		uploads:   NewAdminUploads(st, manager),
	}
}

func (m *Module) MountAdmin(r chi.Router) {
	r.Get("/libraries", m.libraries.List)
	r.Post("/libraries", m.libraries.Create)
	r.Delete("/libraries/{id}", m.libraries.Delete)
	r.Post("/libraries/{id}/scan", m.libraries.Scan)
	r.Post("/libraries/scan-all", m.libraries.ScanAll)

	r.Get("/uploads", m.uploads.List)
	r.Post("/uploads", m.uploads.Create)
	r.Get("/uploads/{id}", m.uploads.Get)
	r.Put("/uploads/{id}", m.uploads.Append)
	r.Post("/uploads/{id}/complete", m.uploads.Complete)
	r.Delete("/uploads/{id}", m.uploads.Abort)
}
