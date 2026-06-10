package transcode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	"couchverse/internal/feature/jobs"
	"couchverse/internal/media"
	"couchverse/internal/settings"
	"couchverse/internal/store"
)

type JobHandler struct {
	Store      *store.Store
	Settings   *settings.Store
	Jobs       *jobs.Store
	DataDir    string
	FFmpegPath string
}

type Payload struct {
	MediaFileID string `json:"mediaFileId"`
	Variant     string `json:"variant"` // "source" (copy-remux) or a rendition name
}

func (h *JobHandler) outDir(mediaFileID, variant string) string {
	return filepath.Join(h.DataDir, "cache", "hls", mediaFileID, variant)
}

func (h *JobHandler) Handle(ctx context.Context, job *jobs.Job, report func(int)) error {
	var p Payload
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		return err
	}
	mf, err := h.Store.MediaFileByID(ctx, p.MediaFileID)
	if err != nil {
		return err
	}
	lib, err := h.Store.LibraryByID(ctx, mf.LibraryID)
	if err != nil {
		return err
	}

	settings := media.LoadTranscodeSettings(ctx, h.Settings)
	spec := BuildSpec{
		Input:          filepath.Join(lib.Path, mf.Path),
		OutDir:         h.outDir(mf.ID, p.Variant),
		HasAudio:       mf.AudioCodec != "",
		Preset:         settings.Preset,
		BackgroundNice: true,
	}

	var variant *store.TranscodeVariant
	if p.Variant == "source" {
		spec.Mode = "copy"
		spec.Rendition = media.Rendition{Name: "source", Height: mf.Height, AudioBitrate: 192_000}
		variant, err = h.Store.UpsertVariant(ctx, mf.ID, "source", mf.Height, mf.Bitrate, 192_000, "copy")
	} else {
		r, ok := media.Renditions[p.Variant]
		if !ok {
			return fmt.Errorf("unknown rendition %q", p.Variant)
		}
		spec.Mode = "transcode"
		spec.Rendition = r
		spec.Encoder = PickEncoder(h.FFmpegPath, settings.HWAccel)
		variant, err = h.Store.UpsertVariant(ctx, mf.ID, r.Name, r.Height, r.VideoBitrate, r.AudioBitrate, "transcode")
	}
	if err != nil {
		return err
	}

	if err := os.MkdirAll(spec.OutDir, 0o755); err != nil {
		return err
	}
	if err := h.Store.SetVariantStatus(ctx, variant.ID, "processing", ""); err != nil {
		return err
	}

	if err := Run(ctx, h.FFmpegPath, spec, mf.DurationSeconds, report); err != nil {
		os.RemoveAll(spec.OutDir)
		finishCtx := context.WithoutCancel(ctx)
		status := "failed"
		if errors.Is(err, context.Canceled) {
			status = "queued" // shutdown/cancel: leave it retryable
		}
		_ = h.Store.SetVariantStatus(finishCtx, variant.ID, status, "")
		return err
	}

	rel := filepath.Join("cache", "hls", mf.ID, p.Variant, "index.m3u8")
	if err := h.Store.SetVariantStatus(ctx, variant.ID, "ready", rel); err != nil {
		return err
	}
	h.maybeDeleteSource(ctx, mf, lib.Path, job.ID, settings)
	return nil
}

// maybeDeleteSource removes the original file once every requested variant is
// ready and nothing else still reads it. Failures only log — the variant this
// job produced is already ready.
func (h *JobHandler) maybeDeleteSource(ctx context.Context, mf *store.MediaFile, libPath string, jobID int64, settings media.TranscodeSettings) {
	if !settings.DeleteSourceEnabled() || mf.SourceDeletedAt != nil || mf.VideoCodec == "" {
		return
	}
	if allReady, err := h.Store.AllVariantsReady(ctx, mf.ID); err != nil || !allReady {
		return
	}
	if busy, err := h.Jobs.HasOtherPendingJobsForMediaFile(ctx, mf.ID, jobID); err != nil || busy {
		return
	}
	src := filepath.Join(libPath, mf.Path)
	if err := os.Remove(src); err != nil && !errors.Is(err, fs.ErrNotExist) {
		slog.Warn("post-transcode cleanup: remove source", "path", src, "err", err)
		return
	}
	if err := h.Store.MarkSourceDeleted(ctx, mf.ID); err != nil {
		slog.Warn("post-transcode cleanup: mark deleted", "mediaFileId", mf.ID, "err", err)
		return
	}
	slog.Info("post-transcode cleanup: removed source", "path", src, "mediaFileId", mf.ID)
}

// RemoveVariant deletes the variant row and its segment directory.
func (h *JobHandler) RemoveVariant(ctx context.Context, id string) error {
	variant, err := h.Store.DeleteVariant(ctx, id)
	if err != nil {
		return err
	}
	return os.RemoveAll(h.outDir(variant.MediaFileID, variant.Name))
}
