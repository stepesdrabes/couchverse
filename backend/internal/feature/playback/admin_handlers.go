package playback

import (
	"context"
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

// TranscodeInfo is what the transcode settings panel shows: the hardware
// encoders found so far, the saved settings and the renditions to pick from.
type TranscodeInfo struct {
	DetectedEncoders []string                `json:"detectedEncoders"`
	Detecting        bool                    `json:"detecting" doc:"Encoder detection is still running; detectedEncoders may grow."`
	Settings         media.TranscodeSettings `json:"settings"`
	Renditions       []string                `json:"renditions"`
}

type transcodeInfoOutput struct{ Body TranscodeInfo }

// Info exposes detected encoders and current transcode settings. Encoder
// detection runs in the background at startup; report progress rather than
// blocking on it.
func (h *AdminTranscode) Info(ctx context.Context, _ *struct{}) (*transcodeInfoOutput, error) {
	encoders, done := DetectedEncoders()
	if encoders == nil {
		encoders = []string{}
	}
	return &transcodeInfoOutput{Body: TranscodeInfo{
		DetectedEncoders: encoders,
		Detecting:        !done,
		Settings:         media.LoadTranscodeSettings(ctx, h.settings),
		Renditions:       []string{"1080p", "720p", "480p"},
	}}, nil
}

type activeTranscodesOutput struct{ Body []jobs.ActiveTranscode }

// Active lists pending/running transcode jobs with their content references.
func (h *AdminTranscode) Active(ctx context.Context, _ *struct{}) (*activeTranscodesOutput, error) {
	active, err := h.jobs.ActiveTranscodes(ctx)
	if err != nil {
		return nil, err
	}
	return &activeTranscodesOutput{Body: active}, nil
}

type idInput struct {
	ID string `path:"id" format:"uuid"`
}

type TranscodeRequest struct {
	Variants []string `json:"variants" required:"false" enum:"1080p,720p,480p,source" doc:"Renditions to queue (source remuxes without re-encoding); the configured ladder when empty."`
}

type enqueueTranscodeInput struct {
	ID   string `path:"id" format:"uuid"`
	Body *TranscodeRequest
}

// QueuedTranscodes names the renditions that were queued; renditions taller
// than the source are skipped rather than upscaled.
type QueuedTranscodes struct {
	Queued []string `json:"queued"`
}

type enqueueTranscodeOutput struct{ Body QueuedTranscodes }

// Enqueue queues HLS variants for a media file.
func (h *AdminTranscode) Enqueue(ctx context.Context, in *enqueueTranscodeInput) (*enqueueTranscodeOutput, error) {
	mf, err := h.library.MediaFileByID(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	if mf.VideoCodec == "" {
		return nil, httpx.BadRequestError("only video files can be transcoded")
	}
	if mf.SourceDeletedAt != nil {
		return nil, httpx.Fail(http.StatusConflict, "source_deleted",
			"the original file was removed after transcoding; re-transcoding is not possible")
	}

	var variants []string
	if in.Body != nil {
		variants = in.Body.Variants
	}
	if len(variants) == 0 {
		variants = media.LoadTranscodeSettings(ctx, h.settings).Ladder
	}

	for _, name := range variants {
		if _, ok := media.Renditions[name]; !ok && name != media.VariantSource {
			return nil, httpx.BadRequestError("unknown rendition " + name)
		}
	}
	tracks, err := h.library.AudioStreamsForFile(ctx, mf.ID)
	if err != nil {
		return nil, err
	}
	queued, err := library.Prepare(ctx, h.library, h.jobs, mf, len(tracks) > 0, variants)
	if err != nil {
		return nil, err
	}
	return &enqueueTranscodeOutput{Body: QueuedTranscodes{Queued: queued}}, nil
}

type variantsOutput struct{ Body []library.TranscodeVariant }

func (h *AdminTranscode) ListVariants(ctx context.Context, in *idInput) (*variantsOutput, error) {
	variants, err := h.library.VariantsForMediaFile(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	return &variantsOutput{Body: variants}, nil
}

func (h *AdminTranscode) DeleteVariant(ctx context.Context, in *idInput) (*struct{}, error) {
	return nil, h.jobHandler.RemoveVariant(ctx, in.ID)
}
