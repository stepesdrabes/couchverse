package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"couchverse/internal/feature/auth"
	"couchverse/internal/feature/jobs"
	"couchverse/internal/feature/library"
	"couchverse/internal/media"
)

// cleanupHandler is the hourly housekeeping job: expired upload sessions,
// stale finished jobs, expired auth sessions and orphaned HLS caches.
// It reschedules itself at the end of every run.
func cleanupHandler(lib *library.Store, au *auth.Store, jb *jobs.Store, uploads *library.Manager, dataDir string) func(context.Context, *jobs.Job, func(int)) error {
	return func(ctx context.Context, _ *jobs.Job, _ func(int)) error {
		if n, err := uploads.Reap(ctx); err != nil {
			return err
		} else if n > 0 {
			slog.Info("cleanup: reaped upload sessions", "count", n)
		}

		if n, err := jb.DeleteOldJobs(ctx, 7*24*time.Hour); err != nil {
			return err
		} else if n > 0 {
			slog.Info("cleanup: pruned old jobs", "count", n)
		}

		if n, err := au.DeleteExpiredSessions(ctx); err != nil {
			return err
		} else if n > 0 {
			slog.Info("cleanup: deleted expired sessions", "count", n)
		}

		if err := removeOrphanedHLS(ctx, lib, dataDir); err != nil {
			return err
		}

		if err := backfillVariantSizes(ctx, lib, dataDir); err != nil {
			return err
		}

		_, err := jb.EnqueueJob(ctx, "cleanup", struct{}{}, jobs.EnqueueOpts{
			RunAt: time.Now().Add(time.Hour),
		})
		return err
	}
}

// backfillVariantSizes measures segment directories of ready variants that
// predate size tracking. Variants whose directory is gone stay at 0.
func backfillVariantSizes(ctx context.Context, lib *library.Store, dataDir string) error {
	variants, err := lib.VariantsMissingSize(ctx, 500)
	if err != nil {
		return err
	}
	for _, v := range variants {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		size := media.DirSize(filepath.Join(dataDir, "cache", "hls", v.MediaFileID, v.Name))
		if size == 0 {
			slog.Warn("cleanup: ready variant has no segments on disk", "variantId", v.ID, "mediaFileId", v.MediaFileID, "name", v.Name)
			continue
		}
		if err := lib.SetVariantSize(ctx, v.ID, size); err != nil {
			return err
		}
	}
	if len(variants) > 0 {
		slog.Info("cleanup: backfilled variant sizes", "count", len(variants))
	}
	return nil
}

// removeOrphanedHLS deletes cache/hls/<id> directories whose media file is gone.
func removeOrphanedHLS(ctx context.Context, lib *library.Store, dataDir string) error {
	hlsDir := filepath.Join(dataDir, "cache", "hls")
	entries, err := os.ReadDir(hlsDir)
	if err != nil {
		return nil // cache dir may not exist yet
	}
	for _, entry := range entries {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		id := entry.Name()
		if len(id) != 36 {
			continue
		}
		if _, err := lib.MediaFileByID(ctx, id); err != nil {
			slog.Info("cleanup: removing orphaned hls cache", "mediaFileId", id)
			os.RemoveAll(filepath.Join(hlsDir, entry.Name()))
		}
	}
	return nil
}
