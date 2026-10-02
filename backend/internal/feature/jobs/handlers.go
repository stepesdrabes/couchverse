package jobs

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"couchverse/internal/httpx"
)

type AdminJobs struct {
	store *Store
}

func NewAdminJobs(st *Store) *AdminJobs {
	return &AdminJobs{store: st}
}

const tag httpx.Tag = "jobs"

func (h *AdminJobs) Register(rt httpx.Routes) {
	huma.Register(rt.Admin, tag.Op("adminListJobs", http.MethodGet, "/jobs"), h.List)
	huma.Register(rt.Admin, tag.NoContent("adminRetryJob", http.MethodPost, "/jobs/{id}/retry"), h.Retry)
	huma.Register(rt.Admin, tag.NoContent("adminCancelJob", http.MethodPost, "/jobs/{id}/cancel"), h.Cancel)
}

type listJobsInput struct {
	Status      string `query:"status" enum:"pending,running,done,failed,cancelled" doc:"Only jobs in this state; all when omitted."`
	MediaFileID string `query:"mediaFileId" format:"uuid" doc:"Only jobs working on this media file."`
	Limit       int    `query:"limit" minimum:"1" maximum:"200" default:"50"`
}

type jobsOutput struct{ Body []AdminJob }

func (h *AdminJobs) List(ctx context.Context, in *listJobsInput) (*jobsOutput, error) {
	jobs, err := h.store.ListJobs(ctx, in.Status, in.MediaFileID, in.Limit)
	if err != nil {
		return nil, err
	}
	return &jobsOutput{Body: jobs}, nil
}

type jobIDInput struct {
	ID int64 `path:"id"`
}

// Retry requeues a failed or cancelled job; any other job is not found.
func (h *AdminJobs) Retry(ctx context.Context, in *jobIDInput) (*struct{}, error) {
	return nil, h.store.RetryJob(ctx, in.ID)
}

// Cancel stops a pending or running job; any other job is not found.
func (h *AdminJobs) Cancel(ctx context.Context, in *jobIDInput) (*struct{}, error) {
	return nil, h.store.CancelJob(ctx, in.ID)
}
