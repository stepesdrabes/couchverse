package library

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"couchverse/internal/feature/catalog"
	"couchverse/internal/feature/jobs"
	"couchverse/internal/media"
	"couchverse/internal/settings"
)

type Prober struct {
	Files       *Store
	Catalog     *catalog.Store
	Settings    *settings.Store
	Jobs        *jobs.Store
	FFprobePath string
}

type ProbePayload struct {
	MediaFileID string `json:"mediaFileId"`
}

// Handle analyzes one media file with ffprobe and attaches it to the catalog,
// matching unassigned video files by filename conventions (creating draft titles).
func (p *Prober) Handle(ctx context.Context, job *jobs.Job, report func(int)) error {
	var payload ProbePayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return err
	}
	mf, err := p.Files.MediaFileByID(ctx, payload.MediaFileID)
	if err != nil {
		return fmt.Errorf("media file %s: %w", payload.MediaFileID, err)
	}
	lib, err := p.Files.LibraryByID(ctx, mf.LibraryID)
	if err != nil {
		return err
	}
	abs := filepath.Join(lib.Path, mf.Path)

	res, err := media.Probe(ctx, p.FFprobePath, abs)
	if err != nil {
		return err
	}
	report(50)

	up := ProbeUpdate{
		Container:       res.Container,
		VideoCodec:      res.VideoCodec,
		AudioCodec:      res.AudioCodec,
		Width:           res.Width,
		Height:          res.Height,
		DurationSeconds: res.DurationSeconds,
		Bitrate:         res.Bitrate,
		Channels:        res.Channels,
		SampleRate:      res.SampleRate,
		VideoRange:      res.VideoRange,
		DirectPlay:      media.DirectPlay(res),
		Probe:           res.Raw,
		// manual assignments (e.g. from uploads) are kept
		TitleID:   mf.TitleID,
		EpisodeID: mf.EpisodeID,
	}

	if mf.TitleID == nil && mf.EpisodeID == nil && res.HasVideo {
		if err := p.assignVideo(ctx, lib, mf.Path, &up); err != nil {
			return err
		}
	}

	if err := p.Files.ApplyProbe(ctx, mf.ID, up); err != nil {
		return err
	}
	if res.HasVideo {
		if err := p.Files.ReplaceAudioStreams(ctx, mf.ID, res.AudioStreams); err != nil {
			return err
		}
	}

	if res.HasVideo && media.HasTextSubtitles(res) {
		if _, err := p.Jobs.EnqueueJobOnce(ctx, "extract_subtitles",
			map[string]string{"mediaFileId": mf.ID}, jobs.EnqueueOpts{}); err != nil {
			return err
		}
	}

	// make non-browser-playable files streamable without admin intervention.
	// Variant rows are created up front so the admin library shows "Processing".
	// multi-audio h264 files always go through a single var_stream_map HLS remux
	// (copied video + every audio language) so the player can switch audio -
	// browsers can't switch the audio of a progressive file. Single-audio files
	// keep the normal path: direct play, or a copy-remux + ladder when needed.
	switch {
	case res.HasVideo && res.VideoCodec == "h264" && len(res.AudioStreams) >= 2:
		if _, err := p.Files.UpsertVariant(ctx, mf.ID, "multiaudio", res.Height, res.Bitrate, 0, "copy"); err != nil {
			return err
		}
		if _, err := p.Jobs.EnqueueJobOnce(ctx, "transcode_hls",
			map[string]any{"mediaFileId": mf.ID, "variant": "multiaudio"}, jobs.EnqueueOpts{}); err != nil {
			return err
		}
	case res.HasVideo && !up.DirectPlay:
		settings := media.LoadTranscodeSettings(ctx, p.Settings)
		if res.VideoCodec == "h264" {
			// h264 streams as-is via a cheap copy-remux (full source quality)...
			source := media.Rendition{Name: "source", Height: res.Height, VideoBitrate: res.Bitrate, AudioBitrate: 192_000}
			if err := p.prepareVariant(ctx, mf.ID, source, "copy"); err != nil {
				return err
			}
			// ...plus lower ladder rungs for adaptive streaming when auto-prepare
			// is on. The source already covers the top tier, so skip rungs at or
			// above its height.
			if settings.AutoPrepareEnabled() {
				for _, r := range media.PrepareRenditions(settings.Ladder, res.Height) {
					if r.Height >= res.Height {
						continue
					}
					if err := p.prepareVariant(ctx, mf.ID, r.CappedAt(res.Bitrate), "transcode"); err != nil {
						return err
					}
				}
			}
		} else if settings.AutoPrepareEnabled() {
			for _, r := range media.PrepareRenditions(settings.Ladder, res.Height) {
				if err := p.prepareVariant(ctx, mf.ID, r.CappedAt(res.Bitrate), "transcode"); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// prepareVariant registers a variant row and enqueues its HLS job.
func (p *Prober) prepareVariant(ctx context.Context, mediaFileID string, r media.Rendition, mode string) error {
	if _, err := p.Files.UpsertVariant(ctx, mediaFileID, r.Name, r.Height, r.VideoBitrate, r.AudioBitrate, mode); err != nil {
		return err
	}
	opts := jobs.EnqueueOpts{}
	if mode == "transcode" {
		opts.MaxAttempts = 2
	}
	_, err := p.Jobs.EnqueueJobOnce(ctx, "transcode_hls",
		map[string]any{"mediaFileId": mediaFileID, "variant": r.Name}, opts)
	return err
}

func (p *Prober) assignVideo(ctx context.Context, lib *Library, relPath string, up *ProbeUpdate) error {
	parsed := media.ParseVideoPath(relPath)

	if parsed.IsEpisode && lib.Kind != "movies" {
		title, err := p.Catalog.FindOrCreateTitle(ctx, "series", parsed.ShowName, parsed.Year)
		if err != nil {
			return err
		}
		seasonID, err := p.Catalog.FindOrCreateSeason(ctx, title.ID, parsed.Season)
		if err != nil {
			return err
		}
		episodeID, err := p.Catalog.FindOrCreateEpisode(ctx, seasonID, parsed.Episode, parsed.Name)
		if err != nil {
			return err
		}
		up.EpisodeID = &episodeID
		return nil
	}

	title, err := p.Catalog.FindOrCreateTitle(ctx, "movie", parsed.Name, parsed.Year)
	if err != nil {
		return err
	}
	up.TitleID = &title.ID
	return nil
}
