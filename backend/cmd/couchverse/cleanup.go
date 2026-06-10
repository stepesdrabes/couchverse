package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"couchverse/internal/feature/auth"
	"couchverse/internal/feature/jobs"
	"couchverse/internal/store"
	"couchverse/internal/upload"
)

// cleanupHandler is the hourly housekeeping job: expired upload sessions,
// stale finished jobs, expired auth sessions and orphaned HLS caches.
// It reschedules itself at the end of every run.
func cleanupHandler(st *store.Store, au *auth.Store, jb *jobs.Store, uploads *upload.Manager, dataDir string) func(context.Context, *jobs.Job, func(int)) error {
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

		if err := removeOrphanedHLS(ctx, st, dataDir); err != nil {
			return err
		}

		_, err := jb.EnqueueJob(ctx, "cleanup", struct{}{}, jobs.EnqueueOpts{
			RunAt: time.Now().Add(time.Hour),
		})
		return err
	}
}

// removeOrphanedHLS deletes cache/hls/<id> directories whose media file is gone.
func removeOrphanedHLS(ctx context.Context, st *store.Store, dataDir string) error {
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
		if _, err := st.MediaFileByID(ctx, id); err != nil {
			slog.Info("cleanup: removing orphaned hls cache", "mediaFileId", id)
			os.RemoveAll(filepath.Join(hlsDir, entry.Name()))
		}
	}
	return nil
}
