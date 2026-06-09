package api

import (
	"net/http"

	"couchverse/internal/httpx"
	"couchverse/internal/store"
	"couchverse/internal/transcode"
)

type AdminTranscode struct {
	store      *store.Store
	jobHandler *transcode.JobHandler
	ffmpegPath string
}

func NewAdminTranscode(st *store.Store, jobHandler *transcode.JobHandler, ffmpegPath string) *AdminTranscode {
	return &AdminTranscode{store: st, jobHandler: jobHandler, ffmpegPath: ffmpegPath}
}

// Info exposes detected encoders and current transcode settings.
func (h *AdminTranscode) Info(w http.ResponseWriter, r *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]any{
		"detectedEncoders": transcode.DetectEncoders(h.ffmpegPath),
		"settings":         transcode.LoadSettings(r.Context(), h.store),
		"renditions":       []string{"1080p", "720p", "480p"},
	})
}

// Enqueue queues HLS variants for a media file.
func (h *AdminTranscode) Enqueue(w http.ResponseWriter, r *http.Request) {
	mediaFileID := httpx.ID(r, "id")
	mf, err := h.store.MediaFileByID(r.Context(), mediaFileID)
	if err != nil {
		respondStoreErr(w, err)
		return
	}
	if mf.VideoCodec == "" {
		httpx.BadRequest(w, "only video files can be transcoded")
		return
	}

	var req struct {
		Variants []string `json:"variants"`
	}
	if err := httpx.Decode(r, &req); err != nil || len(req.Variants) == 0 {
		// default: the configured ladder
		req.Variants = transcode.LoadSettings(r.Context(), h.store).Ladder
	}

	queued := []string{}
	for _, name := range req.Variants {
		rendition, ok := transcode.Renditions[name]
		if !ok && name != "source" {
			httpx.BadRequest(w, "unknown rendition "+name)
			return
		}
		// skip upscaling renditions taller than the source
		if ok && mf.Height > 0 && rendition.Height > mf.Height {
			continue
		}
		mode := "transcode"
		height := rendition.Height
		var vbr, abr int64 = rendition.VideoBitrate, rendition.AudioBitrate
		if name == "source" {
			mode, height, vbr, abr = "copy", mf.Height, mf.Bitrate, 192_000
		}
		if _, err := h.store.UpsertVariant(r.Context(), mf.ID, name, height, vbr, abr, mode); err != nil {
			httpx.Internal(w, err)
			return
		}
		if _, err := h.store.EnqueueJobOnce(r.Context(), "transcode_hls",
			transcode.Payload{MediaFileID: mf.ID, Variant: name},
			store.EnqueueOpts{MaxAttempts: 2}); err != nil {
			httpx.Internal(w, err)
			return
		}
		queued = append(queued, name)
	}
	httpx.JSON(w, http.StatusAccepted, map[string]any{"queued": queued})
}

func (h *AdminTranscode) ListVariants(w http.ResponseWriter, r *http.Request) {
	variants, err := h.store.VariantsForMediaFile(r.Context(), httpx.ID(r, "id"))
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, variants)
}

func (h *AdminTranscode) DeleteVariant(w http.ResponseWriter, r *http.Request) {
	if err := h.jobHandler.RemoveVariant(r.Context(), httpx.ID(r, "id")); err != nil {
		respondStoreErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}
