package media

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"couchverse/internal/store"
)

type Scanner struct {
	Store *store.Store
}

type ScanPayload struct {
	LibraryID int64 `json:"libraryId"`
}

// Handle walks a library folder, registers new/changed files and enqueues a
// probe job for each; rows whose file vanished are removed.
func (sc *Scanner) Handle(ctx context.Context, job *store.Job, report func(int)) error {
	var p ScanPayload
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		return err
	}
	lib, err := sc.Store.LibraryByID(ctx, p.LibraryID)
	if err != nil {
		return fmt.Errorf("library %d: %w", p.LibraryID, err)
	}

	existing, err := sc.Store.MediaFileStubsByLibrary(ctx, lib.ID)
	if err != nil {
		return err
	}

	seen := map[string]bool{}
	queued := 0
	err = filepath.WalkDir(lib.Path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		name := d.Name()
		if strings.HasPrefix(name, ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(name))
		if lib.Kind == "music" {
			if !IsAudioFile(ext) {
				return nil
			}
		} else if !IsVideoFile(ext) {
			return nil
		}

		rel, err := filepath.Rel(lib.Path, path)
		if err != nil {
			return err
		}
		seen[rel] = true

		info, err := d.Info()
		if err != nil {
			return err
		}

		// sub-second tolerance: Postgres stores µs, filesystems report ns
		if stub, ok := existing[rel]; ok &&
			stub.SizeBytes == info.Size() &&
			stub.FileMtime != nil && stub.FileMtime.Sub(info.ModTime()).Abs() < time.Second {
			return nil
		}

		id, err := sc.Store.UpsertMediaFileStub(ctx, lib.ID, rel, info.Size(), info.ModTime())
		if err != nil {
			return err
		}
		if _, err := sc.Store.EnqueueJobOnce(ctx, "probe", ProbePayload{MediaFileID: id}, store.EnqueueOpts{}); err != nil {
			return err
		}
		queued++
		return nil
	})
	if err != nil {
		return err
	}

	removed := 0
	for path, stub := range existing {
		if !seen[path] {
			if err := sc.Store.DeleteMediaFile(ctx, stub.ID); err != nil {
				return err
			}
			removed++
		}
	}

	slog.Info("library scan finished", "library", lib.Name, "queuedProbes", queued, "removed", removed)
	return sc.Store.TouchLibraryScanned(ctx, lib.ID)
}
