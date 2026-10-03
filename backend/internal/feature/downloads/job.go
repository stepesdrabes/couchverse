package downloads

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"couchverse/internal/db"
	"couchverse/internal/feature/jobs"
	"couchverse/internal/feature/library"
	"couchverse/internal/feature/playback"
	"couchverse/internal/feature/subtitles"
	"couchverse/internal/media"
	"couchverse/internal/settings"
)

// Preparer runs the prepare_download job.
type Preparer struct {
	Store      *Store
	Files      *library.Store
	Subtitles  *subtitles.Service
	Settings   *settings.Store
	DataDir    string
	FFmpegPath string
}

func (p *Preparer) Handle(ctx context.Context, j *jobs.Job, report func(int)) error {
	var payload JobPayload
	if err := json.Unmarshal(j.Payload, &payload); err != nil {
		return err
	}
	f, err := p.Store.FileByID(ctx, payload.FileID)
	if errors.Is(err, db.ErrNotFound) {
		return nil // cleaned up before its turn
	}
	if err != nil {
		return err
	}
	out := FilePath(p.DataDir, f.ID)
	if _, serr := os.Stat(out); f.Status == "ready" && serr == nil {
		return nil
	}
	if wanted, err := p.Store.Wanted(ctx, f.ID); err != nil || !wanted {
		if err != nil {
			return err
		}
		// nobody asks for it anymore; the next cleanup removes the row
		return p.Store.SetFailed(ctx, f.ID, "failed", "no longer requested")
	}

	err = p.prepare(ctx, f, out, report)
	if err == nil {
		return nil
	}
	finishCtx := context.WithoutCancel(ctx)
	status := "failed"
	if errors.Is(err, context.Canceled) {
		status = "queued" // shutdown: the job runs again
	}
	if serr := p.Store.SetFailed(finishCtx, f.ID, status, err.Error()); serr != nil {
		slog.Warn("download: record failure", "fileId", f.ID, "err", serr)
	}
	return err
}

func (p *Preparer) prepare(ctx context.Context, f *File, out string, report func(int)) error {
	mf, err := p.Files.MediaFileByID(ctx, f.MediaFileID)
	if err != nil {
		return err
	}
	if mf.SourceDeletedAt != nil {
		return errors.New("the source file was removed after transcoding")
	}
	lib, err := p.Files.LibraryByID(ctx, mf.LibraryID)
	if err != nil {
		return err
	}
	subs, err := p.subtitleFiles(ctx, mf.ID, f.Plan.Subtitles)
	if err != nil {
		return err
	}
	if err := p.Store.SetPreparing(ctx, f.ID); err != nil {
		return err
	}

	set := media.LoadTranscodeSettings(ctx, p.Settings)
	encoder := playback.PickEncoder(p.FFmpegPath, set.HWAccel)
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	part := strings.TrimSuffix(out, ".mp4") + ".part"
	defer os.Remove(part)
	args := playback.DownloadArgs(mf, filepath.Join(lib.Path, mf.Path), f.Plan.Media, subs, part, encoder, set.Preset, playback.DetectFeatures(p.FFmpegPath))

	last := -1
	progress := func(pct int) {
		report(pct)
		// a row update per percent is plenty for a client polling every few seconds
		if pct != last {
			last = pct
			if err := p.Store.SetProgress(ctx, f.ID, pct); err != nil {
				slog.Warn("download: progress", "fileId", f.ID, "err", err)
			}
		}
	}
	started := time.Now()
	if err := playback.Run(ctx, p.FFmpegPath, args, true, mf.DurationSeconds, progress); err != nil {
		return err
	}
	info, err := os.Stat(part)
	if err != nil {
		return err
	}
	if err := os.Rename(part, out); err != nil {
		return err
	}
	slog.Info("download: prepared", "fileId", f.ID, "mediaFileId", mf.ID, "spec", f.Spec,
		"bytes", info.Size(), "took", time.Since(started).Round(time.Second))
	return p.Store.SetReady(ctx, f.ID, info.Size())
}

// subtitleFiles finds the planned tracks on disk; a track deleted since is left out.
func (p *Preparer) subtitleFiles(ctx context.Context, mediaFileID string, refs []SubtitleRef) ([]playback.DownloadSubtitle, error) {
	rows, err := p.Subtitles.Subs.SubtitlesForMediaFile(ctx, mediaFileID)
	if err != nil {
		return nil, err
	}
	out := []playback.DownloadSubtitle{}
	for _, ref := range refs {
		i := slices.IndexFunc(rows, func(s media.Subtitle) bool { return s.ID == ref.ID })
		if i < 0 || rows[i].Path == "" {
			continue
		}
		path := p.Subtitles.Path(&rows[i])
		if _, err := os.Stat(path); err != nil {
			continue
		}
		out = append(out, playback.DownloadSubtitle{Path: path, Lang: ref.Lang, Label: ref.Label, Forced: ref.Forced})
	}
	return out, nil
}

// Sweep is the downloads part of the hourly cleanup: files past retention,
// failed or no longer asked for go, and so do MP4s on disk without a row.
func Sweep(ctx context.Context, st *Store, dataDir string) error {
	expired, err := st.Expire(ctx, Retention)
	if err != nil {
		return fmt.Errorf("expire downloads: %w", err)
	}
	for _, id := range expired {
		if err := os.Remove(FilePath(dataDir, id)); err != nil && !errors.Is(err, os.ErrNotExist) {
			slog.Warn("cleanup: remove download", "fileId", id, "err", err)
		}
	}
	if len(expired) > 0 {
		slog.Info("cleanup: removed downloads", "count", len(expired))
	}

	dir := filepath.Join(dataDir, "cache", "downloads")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil // nothing prepared yet
	}
	known, err := st.FileIDs(ctx)
	if err != nil {
		return err
	}
	for _, e := range entries {
		id := strings.TrimSuffix(strings.TrimSuffix(e.Name(), ".mp4"), ".part")
		if known[id] {
			continue
		}
		slog.Info("cleanup: removing orphaned download", "name", e.Name())
		os.Remove(filepath.Join(dir, e.Name()))
	}
	return nil
}
