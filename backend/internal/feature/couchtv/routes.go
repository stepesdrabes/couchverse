package couchtv

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"couchverse/internal/httpx"
)

// Module mounts the Couch TV routes.
type Module struct {
	admin  *Admin
	titles *Titles
}

func NewModule(s *Scheduler, a *Analyzer) *Module {
	return &Module{admin: &Admin{scheduler: s}, titles: &Titles{analyzer: a}}
}

const tag httpx.Tag = "couchtv"

// Register adds the channel's operations. The admin ones are deliberately not gated by
// the couchTvEnabled flag: the channel is set up and planned before it goes on air.
func (m *Module) Register(rt httpx.Routes) {
	huma.Register(rt.Admin, tag.Op("adminGetCouchTv", http.MethodGet, "/couchtv"), m.admin.Get)
	huma.Register(rt.Admin, tag.Op("adminUpdateCouchTv", http.MethodPut, "/couchtv"), m.admin.Update)
	huma.Register(rt.Admin, tag.Op("adminListAirings", http.MethodGet, "/couchtv/airings"), m.admin.ListAirings)
	huma.Register(rt.Admin, tag.Op("adminRegenerateDay", http.MethodPost, "/couchtv/days/{date}/regenerate"), m.admin.RegenerateDay)
	huma.Register(rt.Admin, tag.Op("adminGetCouchTvTitle", http.MethodGet, "/couchtv/titles/{id}"), m.titles.Get)
	huma.Register(rt.Admin, tag.Op("adminUpdateCouchTvTitle", http.MethodPut, "/couchtv/titles/{id}"), m.titles.Update)
	huma.Register(rt.Admin, tag.Accepted("adminAnalyzeCouchTvTitle", http.MethodPost, "/couchtv/titles/{id}/analyze"), m.titles.Analyze)
	huma.Register(rt.Admin, tag.Op("adminCouchTvWaveform", http.MethodGet, "/couchtv/media-files/{id}/waveform"), m.titles.Waveform)
}
