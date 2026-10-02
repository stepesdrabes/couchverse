package system

import (
	"context"
	"path/filepath"

	"couchverse/internal/feature/jobs"
	"couchverse/internal/httpx"
	"couchverse/internal/media"
)

type AdminStorage struct {
	store   *Store
	jobs    *jobs.Store
	dataDir string
}

func NewAdminStorage(st *Store, jb *jobs.Store, dataDir string) *AdminStorage {
	return &AdminStorage{store: st, jobs: jb, dataDir: dataDir}
}

// StorageInfo is Couchverse's disk footprint against the space available to it.
type StorageInfo struct {
	DiskTotal  int64             `json:"diskTotal" doc:"Size of the disk holding the data directory; 0 when unknown."`
	Free       int64             `json:"free" doc:"Free bytes on that disk; 0 when unknown."`
	Used       int64             `json:"used" doc:"Bytes Couchverse occupies, the sum of the categories."`
	Budget     int64             `json:"budget" doc:"used + free: the space available to Couchverse."`
	Categories []StorageCategory `json:"categories"`
}

type StorageCategory struct {
	Kind  string `json:"kind" enum:"movies,series,transcodes,cache"`
	Bytes int64  `json:"bytes"`
}

type storageOutput struct{ Body StorageInfo }

// Get reports Couchverse's footprint relative to the space available to it,
// not the whole disk. The denominator ("budget") is Couchverse's own usage
// plus the disk's free space - i.e. everything Couchverse could occupy,
// excluding whatever else already lives on the disk. Usage is broken down by
// category (movies/series/transcodes/cache) for the segmented bar.
func (h *AdminStorage) Get(ctx context.Context, _ *struct{}) (*storageOutput, error) {
	diskTotal, free, ok := diskUsage(h.dataDir)
	if !ok {
		diskTotal, free = 0, 0
	}

	byKind, err := h.store.MediaUsageByKind(ctx)
	if err != nil {
		return nil, err
	}
	transcodes, err := h.store.TranscodeUsage(ctx)
	if err != nil {
		return nil, err
	}
	cache := media.DirSize(filepath.Join(h.dataDir, "cache", "images")) +
		media.DirSize(filepath.Join(h.dataDir, "cache", "uploads")) +
		media.DirSize(filepath.Join(h.dataDir, "cache", "sessions")) +
		media.DirSize(filepath.Join(h.dataDir, "cache", "frames"))

	categories := []StorageCategory{
		{Kind: "movies", Bytes: byKind["movies"]},
		{Kind: "series", Bytes: byKind["series"]},
		{Kind: "transcodes", Bytes: transcodes},
		{Kind: "cache", Bytes: cache},
	}
	var used int64
	for _, c := range categories {
		used += c.Bytes
	}

	return &storageOutput{Body: StorageInfo{
		DiskTotal:  diskTotal,
		Free:       free,
		Used:       used,
		Budget:     used + free,
		Categories: categories,
	}}, nil
}

// DashboardOverview is the admin dashboard's summary of the catalog and queue.
type DashboardOverview struct {
	Counts      DashboardCounts `json:"counts"`
	Library     LibraryInsights `json:"library"`
	PendingJobs int             `json:"pendingJobs" doc:"Jobs pending or running."`
	RecentJobs  []jobs.AdminJob `json:"recentJobs" doc:"The newest jobs, newest first."`
}

type overviewOutput struct{ Body DashboardOverview }

func (h *AdminStorage) Overview(ctx context.Context, _ *struct{}) (*overviewOutput, error) {
	counts, err := h.store.Overview(ctx)
	if err != nil {
		return nil, err
	}
	pending, err := h.jobs.PendingJobCount(ctx)
	if err != nil {
		return nil, err
	}
	recent, err := h.jobs.ListJobs(ctx, "", "", 6)
	if err != nil {
		return nil, err
	}
	library, err := h.store.LibraryStats(ctx)
	if err != nil {
		return nil, err
	}
	return &overviewOutput{Body: DashboardOverview{
		Counts:      *counts,
		Library:     *library,
		PendingJobs: pending,
		RecentJobs:  recent,
	}}, nil
}

// HomeRows handlers (admin-curated home page rows)

type homeRowsOutput struct{ Body []HomeRowConfig }

func (h *AdminStorage) HomeRowsGet(ctx context.Context, _ *struct{}) (*homeRowsOutput, error) {
	rows, err := h.store.ListHomeRows(ctx)
	if err != nil {
		return nil, err
	}
	return &homeRowsOutput{Body: rows}, nil
}

type homeRowsInput struct {
	Body []HomeRowInput
}

// HomeRowsPut replaces the home page layout with the posted rows, in order.
func (h *AdminStorage) HomeRowsPut(ctx context.Context, in *homeRowsInput) (*homeRowsOutput, error) {
	for _, row := range in.Body {
		if row.Kind == "genre" && row.GenreID == nil {
			return nil, httpx.BadRequestError("genre rows need a genreId")
		}
	}
	if err := h.store.ReplaceHomeRows(ctx, in.Body); err != nil {
		return nil, err
	}
	return h.HomeRowsGet(ctx, nil)
}
