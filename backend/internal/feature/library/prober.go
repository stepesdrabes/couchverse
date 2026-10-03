package library

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
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

	res, err := media.Probe(ctx, p.FFprobePath, filepath.Join(lib.Path, mf.Path))
	if err != nil {
		return err
	}
	report(50)

	up := ProbeUpdateFrom(res)
	// manual assignments (e.g. from uploads) are kept
	up.TitleID, up.EpisodeID = mf.TitleID, mf.EpisodeID
	if mf.TitleID == nil && mf.EpisodeID == nil && res.HasVideo {
		if err := p.assignVideo(ctx, lib, mf.Path, &up); err != nil {
			return err
		}
	}

	if err := p.store(ctx, mf.ID, up, res); err != nil {
		return err
	}

	if res.HasVideo && media.HasTextSubtitles(res) {
		if _, err := p.Jobs.EnqueueJobOnce(ctx, "extract_subtitles",
			map[string]string{"mediaFileId": mf.ID}, jobs.EnqueueOpts{}); err != nil {
			return err
		}
	}

	// make the file streamable for every client without admin intervention
	if !res.HasVideo {
		return nil
	}
	probed, err := p.Files.MediaFileByID(ctx, mf.ID)
	if err != nil {
		return err
	}
	names := media.AutoPrepare(res, media.LoadTranscodeSettings(ctx, p.Settings))
	_, err = Prepare(ctx, p.Files, p.Jobs, probed, len(res.AudioStreams) > 0, names)
	return err
}

func (p *Prober) store(ctx context.Context, id string, up ProbeUpdate, res *media.ProbeResult) error {
	if err := p.Files.ApplyProbe(ctx, id, up); err != nil {
		return err
	}
	if res.HasVideo {
		return p.Files.ReplaceAudioStreams(ctx, id, res.AudioStreams)
	}
	return nil
}

// Reprobe brings files an older prober read up to media.ProbeVersion: ffprobe
// again, or the stored probe output once the source is gone (losing only what
// needs a decoded frame, HDR10+). Files that have no HLS v2 package yet get the
// cheap one (the copied source and audio renditions) when auto-prepare is on;
// their ladder is left alone, so an upgrade never re-encodes a library.
func (p *Prober) Reprobe(ctx context.Context, _ *jobs.Job, report func(int)) error {
	settings := media.LoadTranscodeSettings(ctx, p.Settings)
	done := 0
	for {
		files, err := p.Files.StaleProbes(ctx, 50)
		if err != nil {
			return err
		}
		if len(files) == 0 {
			return nil
		}
		for i := range files {
			if err := p.reprobe(ctx, &files[i], settings); err != nil {
				// a file that cannot be read keeps its old facts; mark it so
				// the loop moves on
				slog.Warn("reprobe", "mediaFile", files[i].ID, "err", err)
				if err := p.Files.SetProbeVersion(ctx, files[i].ID, media.ProbeVersion); err != nil {
					return err
				}
			}
			done++
			report(min(99, done))
		}
	}
}

func (p *Prober) reprobe(ctx context.Context, mf *media.MediaFile, settings media.TranscodeSettings) error {
	lib, err := p.Files.LibraryByID(ctx, mf.LibraryID)
	if err != nil {
		return err
	}
	path := filepath.Join(lib.Path, mf.Path)
	var res *media.ProbeResult
	if _, statErr := os.Stat(path); mf.SourceDeletedAt == nil && statErr == nil {
		res, err = media.Probe(ctx, p.FFprobePath, path)
	} else if errors.Is(statErr, fs.ErrNotExist) || mf.SourceDeletedAt != nil {
		res, err = media.ParseProbe(mf.Probe, mf.Path)
	} else {
		err = statErr
	}
	if err != nil {
		return err
	}
	up := ProbeUpdateFrom(res)
	up.TitleID, up.EpisodeID = mf.TitleID, mf.EpisodeID
	if mf.SourceDeletedAt != nil {
		up.DirectPlay = false
	}
	if err := p.store(ctx, mf.ID, up, res); err != nil {
		return err
	}
	if mf.SourceDeletedAt != nil || !settings.AutoPrepareEnabled() {
		return nil
	}
	variants, err := p.Files.VariantsForMediaFile(ctx, mf.ID)
	if err != nil {
		return err
	}
	for _, v := range variants {
		if v.Format == "fmp4" {
			return nil
		}
	}
	for _, name := range media.AutoPrepare(res, settings) {
		if name == media.VariantSource {
			probed, err := p.Files.MediaFileByID(ctx, mf.ID)
			if err != nil {
				return err
			}
			_, err = Prepare(ctx, p.Files, p.Jobs, probed, len(res.AudioStreams) > 0, []string{name})
			return err
		}
	}
	return nil
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
