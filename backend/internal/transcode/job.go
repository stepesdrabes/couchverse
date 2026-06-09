package transcode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"couchverse/internal/store"
)

type JobHandler struct {
	Store      *store.Store
	DataDir    string
	FFmpegPath string
}

type Payload struct {
	MediaFileID int64  `json:"mediaFileId"`
	Variant     string `json:"variant"` // "source" (copy-remux) or a rendition name
}

func (h *JobHandler) outDir(mediaFileID int64, variant string) string {
	return filepath.Join(h.DataDir, "cache", "hls", fmt.Sprint(mediaFileID), variant)
}

func (h *JobHandler) Handle(ctx context.Context, job *store.Job, report func(int)) error {
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

	settings := LoadSettings(ctx, h.Store)
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
		spec.Rendition = Rendition{Name: "source", Height: mf.Height, AudioBitrate: 192_000}
		variant, err = h.Store.UpsertVariant(ctx, mf.ID, "source", mf.Height, mf.Bitrate, 192_000, "copy")
	} else {
		r, ok := Renditions[p.Variant]
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

	rel := filepath.Join("cache", "hls", fmt.Sprint(mf.ID), p.Variant, "index.m3u8")
	return h.Store.SetVariantStatus(ctx, variant.ID, "ready", rel)
}

// RemoveVariant deletes the variant row and its segment directory.
func (h *JobHandler) RemoveVariant(ctx context.Context, id int64) error {
	variant, err := h.Store.DeleteVariant(ctx, id)
	if err != nil {
		return err
	}
	return os.RemoveAll(h.outDir(variant.MediaFileID, variant.Name))
}
