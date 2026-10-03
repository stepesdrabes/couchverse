package playback

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
	"couchverse/internal/feature/library"
	"couchverse/internal/media"
	"couchverse/internal/settings"
)

type JobHandler struct {
	Files      *library.Store
	Settings   *settings.Store
	Jobs       *jobs.Store
	DataDir    string
	FFmpegPath string
}

type Payload struct {
	MediaFileID string `json:"mediaFileId"`
	// Variant is "package" (the copied source video and every audio
	// rendition, in one pass), "trickplay" or a ladder rendition name.
	// "source" and "multiaudio" are the pre-v2 names of the package.
	Variant string `json:"variant"`
}

func (h *JobHandler) hlsDir(mediaFileID string) string {
	return filepath.Join(h.DataDir, "cache", "hls", mediaFileID)
}

// job is one ffmpeg run and the variant rows it fills.
type job struct {
	args     []string
	variants []*library.TranscodeVariant
	// dirs are the rendition directories the run writes, with what to record
	// about each in its rendition.json
	dirs map[string]renditionInfo
}

func (h *JobHandler) Handle(ctx context.Context, j *jobs.Job, report func(int)) error {
	var p Payload
	if err := json.Unmarshal(j.Payload, &p); err != nil {
		return err
	}
	mf, err := h.Files.MediaFileByID(ctx, p.MediaFileID)
	if err != nil {
		return err
	}
	lib, err := h.Files.LibraryByID(ctx, mf.LibraryID)
	if err != nil {
		return err
	}
	settings := media.LoadTranscodeSettings(ctx, h.Settings)
	f := DetectFeatures(h.FFmpegPath)
	input := filepath.Join(lib.Path, mf.Path)

	var run *job
	switch p.Variant {
	case media.VariantPackage, media.VariantSource, "multiaudio":
		run, err = h.packageJob(ctx, mf, input, f)
	case media.VariantTrickplay:
		run, err = h.trickplayJob(ctx, mf, input, f)
	default:
		r, ok := media.Renditions[p.Variant]
		if !ok {
			return fmt.Errorf("unknown rendition %q", p.Variant)
		}
		run, err = h.rungJob(ctx, mf, input, r.CappedAt(mf.Bitrate), PickEncoder(h.FFmpegPath, settings.HWAccel), settings.Preset, f)
	}
	if err != nil {
		return err
	}
	if run == nil {
		return nil
	}

	for dir := range run.dirs {
		os.RemoveAll(dir) // a pre-v2 variant of the same name leaves MPEG-TS behind
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	for _, v := range run.variants {
		if err := h.Files.SetVariantStatus(ctx, v.ID, "processing", "", 0); err != nil {
			return err
		}
	}

	err = Run(ctx, h.FFmpegPath, run.args, true, mf.DurationSeconds, report)
	if err == nil {
		err = h.describe(run)
	}
	if err != nil {
		finishCtx := context.WithoutCancel(ctx)
		status := "failed"
		if errors.Is(err, context.Canceled) {
			status = "queued" // shutdown/cancel: leave it retryable
		}
		for dir := range run.dirs {
			os.RemoveAll(dir)
		}
		for _, v := range run.variants {
			_ = h.Files.SetVariantStatus(finishCtx, v.ID, status, "", 0)
		}
		return err
	}

	for _, v := range run.variants {
		rel := filepath.Join("cache", "hls", mf.ID, v.Name, "index.m3u8")
		if err := h.Files.SetVariantStatus(ctx, v.ID, "ready", rel, h.variantSize(mf.ID, v.Name)); err != nil {
			return err
		}
	}
	h.maybeDeleteSource(ctx, mf, lib.Path, j.ID, settings)
	return nil
}

func (h *JobHandler) describe(run *job) error {
	for dir, base := range run.dirs {
		if _, err := describeRendition(dir, base); err != nil {
			return fmt.Errorf("describe %s: %w", filepath.Base(dir), err)
		}
		if filepath.Base(dir) == media.VariantTrickplay {
			if err := writeIFramePlaylist(dir); err != nil {
				return err
			}
		}
	}
	return nil
}

// packageJob copies the source video (when it can go into fMP4 as is) and
// prepares every audio rendition in one read of the source.
func (h *JobHandler) packageJob(ctx context.Context, mf *media.MediaFile, input string, f Features) (*job, error) {
	tracks, err := h.Files.AudioStreamsForFile(ctx, mf.ID)
	if err != nil {
		return nil, err
	}
	run := &job{args: inputArgs(input, 0), dirs: map[string]renditionInfo{}}
	base := h.hlsDir(mf.ID)
	if canCopySource(mf, f) {
		v, err := h.Files.UpsertVariant(ctx, mf.ID, media.VariantSource, mf.Height, mf.Bitrate, 0, "copy")
		if err != nil {
			return nil, err
		}
		run.variants = append(run.variants, v)
		dir := filepath.Join(base, media.VariantSource)
		run.args = append(run.args, hlsOutput{dir: dir, codec: sourceVideoArgs(mf, f), video: true, segment: segmentSeconds}.args()...)
		run.dirs[dir] = renditionInfo{FrameRate: mf.Video.FrameRate, VideoRange: videoRange(mf.Video)}
	} else if err := h.Files.DeleteVariantByName(ctx, mf.ID, media.VariantSource); err != nil {
		// a source registered by the prober that cannot be copied would read as pending forever
		return nil, err
	}
	if len(tracks) > 0 {
		mode := "copy"
		renditions := audioRenditions(tracks)
		for _, a := range renditions {
			if !a.copy {
				mode = "transcode"
			}
		}
		v, err := h.Files.UpsertVariant(ctx, mf.ID, media.VariantAudio, 0, 0, 0, mode)
		if err != nil {
			return nil, err
		}
		run.variants = append(run.variants, v)
		for _, a := range renditions {
			dir := filepath.Join(base, a.dir())
			run.args = append(run.args, hlsOutput{dir: dir, codec: a.args(), segment: segmentSeconds}.args()...)
			run.dirs[dir] = renditionInfo{StreamIndex: a.track.Index, Language: media.BCP47(a.track.Lang),
				Name: a.track.Title, Default: a.track.Default}
		}
	}
	if len(run.variants) == 0 {
		return nil, nil
	}
	return run, nil
}

func (h *JobHandler) rungJob(ctx context.Context, mf *media.MediaFile, input string, r media.Rendition, encoder, preset string, f Features) (*job, error) {
	v, err := h.Files.UpsertVariant(ctx, mf.ID, r.Name, r.Height, r.VideoBitrate, 0, "transcode")
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(h.hlsDir(mf.ID), r.Name)
	out := hlsOutput{dir: dir, codec: rungVideoArgs(mf, r, encoder, preset, f, 0), video: true, segment: segmentSeconds}
	return &job{
		args:     append(inputArgs(input, 0), out.args()...),
		variants: []*library.TranscodeVariant{v},
		dirs:     map[string]renditionInfo{dir: {FrameRate: mf.Video.FrameRate, VideoRange: "SDR"}},
	}, nil
}

func (h *JobHandler) trickplayJob(ctx context.Context, mf *media.MediaFile, input string, f Features) (*job, error) {
	v, err := h.Files.UpsertVariant(ctx, mf.ID, media.VariantTrickplay, 180, 0, 0, "transcode")
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(h.hlsDir(mf.ID), media.VariantTrickplay)
	out := hlsOutput{dir: dir, codec: trickplayArgs(mf, f), video: true, segment: trickSeconds}
	return &job{
		// only keyframes are decoded; the trick frames do not need more
		args:     append(inputArgs(input, 0, "-skip_frame", "nokey"), out.args()...),
		variants: []*library.TranscodeVariant{v},
		dirs:     map[string]renditionInfo{dir: {VideoRange: "SDR"}},
	}, nil
}

// variantDirs are the directories a variant row owns: audio owns every
// audio-<stream>-<codec> rendition.
func (h *JobHandler) variantDirs(mediaFileID, name string) []string {
	base := h.hlsDir(mediaFileID)
	if name != media.VariantAudio {
		return []string{filepath.Join(base, name)}
	}
	matches, _ := filepath.Glob(filepath.Join(base, "audio-*"))
	return matches
}

func (h *JobHandler) variantSize(mediaFileID, name string) int64 {
	var total int64
	for _, dir := range h.variantDirs(mediaFileID, name) {
		total += media.DirSize(dir)
	}
	return total
}

// maybeDeleteSource removes the original file once every requested variant is
// ready and nothing else still reads it. Failures only log - the variant this
// job produced is already ready.
func (h *JobHandler) maybeDeleteSource(ctx context.Context, mf *media.MediaFile, libPath string, jobID int64, settings media.TranscodeSettings) {
	if !settings.DeleteSourceEnabled() || mf.SourceDeletedAt != nil || mf.VideoCodec == "" {
		return
	}
	if allReady, err := h.Files.AllVariantsReady(ctx, mf.ID); err != nil || !allReady {
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
	if err := h.Files.MarkSourceDeleted(ctx, mf.ID); err != nil {
		slog.Warn("post-transcode cleanup: mark deleted", "mediaFileId", mf.ID, "err", err)
		return
	}
	slog.Info("post-transcode cleanup: removed source", "path", src, "mediaFileId", mf.ID)
}

// RemoveVariant deletes the variant row and its segment directories.
func (h *JobHandler) RemoveVariant(ctx context.Context, id string) error {
	variant, err := h.Files.DeleteVariant(ctx, id)
	if err != nil {
		return err
	}
	for _, dir := range h.variantDirs(variant.MediaFileID, variant.Name) {
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
	}
	return nil
}
