package library

import (
	"context"
	"slices"

	"couchverse/internal/feature/jobs"
	"couchverse/internal/media"
)

// Prepare queues HLS v2 renditions of a file: media.VariantSource for the copied
// video and ladder rendition names. Every HLS presentation needs the audio
// renditions and gets the trick-play rendition, so those come along with
// anything queued. Renditions above the source's class are skipped rather than
// upscaled. Variant rows are registered up front so the admin library shows
// "Processing". It returns the names queued.
func Prepare(ctx context.Context, files *Store, jb *jobs.Store, mf *media.MediaFile, hasAudio bool, names []string) ([]string, error) {
	enqueue := func(variant string, opts jobs.EnqueueOpts) error {
		_, err := jb.EnqueueJobOnce(ctx, "transcode_hls",
			map[string]string{"mediaFileId": mf.ID, "variant": variant}, opts)
		return err
	}
	queued := []string{}
	for _, name := range names {
		r, ok := media.Renditions[name]
		if !ok || mf.Height > 0 && r.Height > media.ClassHeight(mf.Width, mf.Height) {
			continue
		}
		r = r.CappedAt(mf.Bitrate)
		if _, err := files.UpsertVariant(ctx, mf.ID, r.Name, r.Height, r.VideoBitrate, 0, "transcode"); err != nil {
			return nil, err
		}
		if err := enqueue(r.Name, jobs.EnqueueOpts{MaxAttempts: 2}); err != nil {
			return nil, err
		}
		queued = append(queued, r.Name)
	}
	source := slices.Contains(names, media.VariantSource)
	if source {
		if _, err := files.UpsertVariant(ctx, mf.ID, media.VariantSource, mf.Height, mf.Bitrate, 0, "copy"); err != nil {
			return nil, err
		}
		queued = append([]string{media.VariantSource}, queued...)
	}
	if len(queued) == 0 {
		return queued, nil
	}
	if hasAudio {
		if _, err := files.UpsertVariant(ctx, mf.ID, media.VariantAudio, 0, 0, 0, "transcode"); err != nil {
			return nil, err
		}
	}
	if source || hasAudio {
		if err := enqueue(media.VariantPackage, jobs.EnqueueOpts{}); err != nil {
			return nil, err
		}
	}
	if _, err := files.UpsertVariant(ctx, mf.ID, media.VariantTrickplay, 180, 0, 0, "transcode"); err != nil {
		return nil, err
	}
	return queued, enqueue(media.VariantTrickplay, jobs.EnqueueOpts{MaxAttempts: 2})
}
