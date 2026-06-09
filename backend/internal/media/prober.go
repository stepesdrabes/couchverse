package media

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"couchverse/internal/store"
)

type Prober struct {
	Store       *store.Store
	FFprobePath string
	DataDir     string
}

type ProbePayload struct {
	MediaFileID int64 `json:"mediaFileId"`
}

// Handle analyzes one media file with ffprobe and attaches it to the catalog:
// video files are matched by filename conventions (creating draft titles),
// audio files by their tags (creating artists/albums/tracks).
func (p *Prober) Handle(ctx context.Context, job *store.Job, report func(int)) error {
	var payload ProbePayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return err
	}
	mf, err := p.Store.MediaFileByID(ctx, payload.MediaFileID)
	if err != nil {
		return fmt.Errorf("media file %d: %w", payload.MediaFileID, err)
	}
	lib, err := p.Store.LibraryByID(ctx, mf.LibraryID)
	if err != nil {
		return err
	}
	abs := filepath.Join(lib.Path, mf.Path)

	res, err := Probe(ctx, p.FFprobePath, abs)
	if err != nil {
		return err
	}
	report(50)

	up := store.ProbeUpdate{
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
		DirectPlay:      DirectPlay(res),
		Probe:           res.Raw,
		// manual assignments (e.g. from uploads) are kept
		TitleID:   mf.TitleID,
		EpisodeID: mf.EpisodeID,
		TrackID:   mf.TrackID,
	}

	unassigned := mf.TitleID == nil && mf.EpisodeID == nil && mf.TrackID == nil
	if unassigned {
		if res.HasVideo {
			if err := p.assignVideo(ctx, lib, mf.Path, &up); err != nil {
				return err
			}
		} else {
			if err := p.assignAudio(ctx, abs, res, &up); err != nil {
				return err
			}
		}
	}

	if err := p.Store.ApplyProbe(ctx, mf.ID, up); err != nil {
		return err
	}

	if res.HasVideo && HasTextSubtitles(res) {
		if _, err := p.Store.EnqueueJobOnce(ctx, "extract_subtitles",
			map[string]int64{"mediaFileId": mf.ID}, store.EnqueueOpts{}); err != nil {
			return err
		}
	}
	return nil
}

func (p *Prober) assignVideo(ctx context.Context, lib *store.Library, relPath string, up *store.ProbeUpdate) error {
	parsed := ParseVideoPath(relPath)

	if parsed.IsEpisode && lib.Kind != "movies" {
		title, err := p.Store.FindOrCreateTitle(ctx, "series", parsed.ShowName, parsed.Year)
		if err != nil {
			return err
		}
		seasonID, err := p.Store.FindOrCreateSeason(ctx, title.ID, parsed.Season)
		if err != nil {
			return err
		}
		episodeID, err := p.Store.FindOrCreateEpisode(ctx, seasonID, parsed.Episode, parsed.Name)
		if err != nil {
			return err
		}
		up.EpisodeID = &episodeID
		return nil
	}

	title, err := p.Store.FindOrCreateTitle(ctx, "movie", parsed.Name, parsed.Year)
	if err != nil {
		return err
	}
	up.TitleID = &title.ID
	return nil
}

func (p *Prober) assignAudio(ctx context.Context, abs string, res *ProbeResult, up *store.ProbeUpdate) error {
	tags := ReadAudioTags(abs)

	artistID, err := p.Store.UpsertArtist(ctx, tags.Artist)
	if err != nil {
		return err
	}
	albumID, err := p.Store.UpsertAlbum(ctx, artistID, tags.Album, tags.Year)
	if err != nil {
		return err
	}
	trackID, err := p.Store.UpsertTrack(ctx, albumID, tags.Disc, tags.Track, tags.Title,
		int(res.DurationSeconds), tags.TrackArtist)
	if err != nil {
		return err
	}
	up.TrackID = &trackID

	if tags.Genre != "" {
		if err := p.Store.SetAlbumGenre(ctx, albumID, tags.Genre); err != nil {
			return err
		}
	}
	if len(tags.Picture) > 0 {
		if err := p.saveAlbumCover(ctx, albumID, tags.Picture, tags.PictureExt); err != nil {
			return err
		}
	}
	return nil
}

// saveAlbumCover stores embedded cover art once per album.
func (p *Prober) saveAlbumCover(ctx context.Context, albumID int64, data []byte, ext string) error {
	existing, err := p.Store.ArtworkFor(ctx, "album", albumID)
	if err != nil {
		return err
	}
	for _, a := range existing {
		if a.Kind == "album_cover" {
			return nil
		}
	}

	rel := filepath.Join("artwork", "album", fmt.Sprint(albumID), "album_cover"+ext)
	abs := filepath.Join(p.DataDir, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(abs, data, 0o644); err != nil {
		return err
	}
	_, err = p.Store.SetArtwork(ctx, "album", albumID, "album_cover", rel, 0, 0, "embedded")
	return err
}
