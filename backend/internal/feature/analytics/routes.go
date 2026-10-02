package analytics

import (
	"context"
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

const tag httpx.Tag = "analytics"

func (m *Module) Register(rt httpx.Routes) {
	huma.Register(rt.Admin, tag.Op("adminGetAnalytics", http.MethodGet, "/analytics/overview"), m.overview)
}

type overviewInput struct {
	// Out-of-range windows are clamped rather than rejected, so the bounds are
	// documented but not enforced as a schema constraint.
	Days int `query:"days" default:"30" doc:"Window in days ending today, clamped to 1..365."`
}

type overviewOutput struct{ Body *AnalyticsOverview }

func (m *Module) overview(ctx context.Context, in *overviewInput) (*overviewOutput, error) {
	out, err := m.store.Overview(ctx, min(max(in.Days, 1), 365))
	if err != nil {
		return nil, err
	}
	return &overviewOutput{Body: out}, nil
}
