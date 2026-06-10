package playback

import (
	"net/http"

	"couchverse/internal/feature/jobs"
	"couchverse/internal/feature/library"
	"couchverse/internal/httpx"
	"couchverse/internal/media"
	"couchverse/internal/settings"
)

type AdminTranscode struct {
	library    *library.Store
	settings   *settings.Store
	jobs       *jobs.Store
	jobHandler *JobHandler
	ffmpegPath string
}

func NewAdminTranscode(lib *library.Store, set *settings.Store, jb *jobs.Store, jobHandler *JobHandler, ffmpegPath string) *AdminTranscode {
	return &AdminTranscode{library: lib, settings: set, jobs: jb, jobHandler: jobHandler, ffmpegPath: ffmpegPath}
}

// Info exposes detected encoders and current transcode settings. Encoder
// detection runs in the background at startup; report progress rather than
// blocking on it.
func (h *AdminTranscode) Info(w http.ResponseWriter, r *http.Request) {
	encoders, done := DetectedEncoders()
	if encoders == nil {
		encoders = []string{}
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"detectedEncoders": encoders,
		"detecting":        !done,
		"settings":         media.LoadTranscodeSettings(r.Context(), h.settings),
		"renditions":       []string{"1080p", "720p", "480p"},
	})
}

// Active lists pending/running transcode jobs with their content references.
func (h *AdminTranscode) Active(w http.ResponseWriter, r *http.Request) {
	active, err := h.jobs.ActiveTranscodes(r.Context())
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, active)
}

// Enqueue queues HLS variants for a media file.
func (h *AdminTranscode) Enqueue(w http.ResponseWriter, r *http.Request) {
	mediaFileID := httpx.UUID(r, "id")
	if mediaFileID == "" {
		httpx.NotFound(w)
		return
	}
	mf, err := h.library.MediaFileByID(r.Context(), mediaFileID)
	if err != nil {
		httpx.StoreErr(w, err)
		return
	}
	if mf.VideoCodec == "" {
		httpx.BadRequest(w, "only video files can be transcoded")
		return
	}
	if mf.SourceDeletedAt != nil {
		httpx.Error(w, http.StatusConflict, "source_deleted",
			"the original file was removed after transcoding; re-transcoding is not possible")
		return
	}

	var req struct {
		Variants []string `json:"variants"`
	}
	if err := httpx.Decode(r, &req); err != nil || len(req.Variants) == 0 {
		// default: the configured ladder
		req.Variants = media.LoadTranscodeSettings(r.Context(), h.settings).Ladder
	}

	queued := []string{}
	for _, name := range req.Variants {
		rendition, ok := media.Renditions[name]
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
		if _, err := h.library.UpsertVariant(r.Context(), mf.ID, name, height, vbr, abr, mode); err != nil {
			httpx.Internal(w, err)
			return
		}
		if _, err := h.jobs.EnqueueJobOnce(r.Context(), "transcode_hls",
			Payload{MediaFileID: mf.ID, Variant: name},
			jobs.EnqueueOpts{MaxAttempts: 2}); err != nil {
			httpx.Internal(w, err)
			return
		}
		queued = append(queued, name)
	}
	httpx.JSON(w, http.StatusAccepted, map[string]any{"queued": queued})
}

func (h *AdminTranscode) ListVariants(w http.ResponseWriter, r *http.Request) {
	mediaFileID := httpx.UUID(r, "id")
	if mediaFileID == "" {
		httpx.NotFound(w)
		return
	}
	variants, err := h.library.VariantsForMediaFile(r.Context(), mediaFileID)
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, variants)
}

func (h *AdminTranscode) DeleteVariant(w http.ResponseWriter, r *http.Request) {
	id := httpx.UUID(r, "id")
	if id == "" {
		httpx.NotFound(w)
		return
	}
	if err := h.jobHandler.RemoveVariant(r.Context(), id); err != nil {
		httpx.StoreErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}
