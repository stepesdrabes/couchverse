package analytics

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"couchverse/internal/httpx"
)

type Module struct {
	store *Store
}

func NewModule(st *Store) *Module {
	return &Module{store: st}
}

func (m *Module) Register(rt httpx.Routes) {
	httpx.Raw(rt.Admin, huma.Operation{OperationID: "adminGetAnalytics", Method: http.MethodGet, Path: "/analytics/overview", Tags: []string{"analytics"}}, m.overview)
}

func (m *Module) overview(w http.ResponseWriter, r *http.Request) {
	days := httpx.QueryInt(r, "days", 30)
	if days < 1 {
		days = 1
	}
	if days > 365 {
		days = 365
	}
	out, err := m.store.Overview(r.Context(), days)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}
