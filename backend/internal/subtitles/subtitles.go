// Package subtitles extracts embedded text subtitles to WebVTT and converts
// uploaded .srt files — players get side-car <track> elements either way.
package subtitles

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"couchverse/internal/feature/jobs"
	"couchverse/internal/feature/library"
	"couchverse/internal/media"
	"couchverse/internal/store"
)

type Service struct {
	Store      *store.Store
	Files      *library.Store
	DataDir    string
	FFmpegPath string
}

func (s *Service) dir(mediaFileID string) string {
	return filepath.Join(s.DataDir, "subtitles", mediaFileID)
}

func (s *Service) Path(sub *store.Subtitle) string {
	return filepath.Join(s.DataDir, sub.Path)
}

// SaveUpload converts an uploaded .srt/.vtt to WebVTT and registers it.
func (s *Service) SaveUpload(ctx context.Context, mediaFileID string, lang, label, filename string, body io.Reader) (*store.Subtitle, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext != ".srt" && ext != ".vtt" {
		return nil, fmt.Errorf("only .srt and .vtt files are supported")
	}
	if lang == "" {
		lang = "und"
	}
	if label == "" {
		label = strings.ToUpper(lang)
	}

	tmp, err := os.CreateTemp("", "couchverse-sub-*"+ext)
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, io.LimitReader(body, 10<<20)); err != nil {
		tmp.Close()
		return nil, err
	}
	tmp.Close()

	sub, err := s.Store.CreateSubtitle(ctx, mediaFileID, lang, label, "uploaded", false, "pending")
	if err != nil {
		return nil, err
	}

	rel := filepath.Join("subtitles", mediaFileID, sub.ID+".vtt")
	abs := filepath.Join(s.DataDir, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return nil, err
	}

	if ext == ".vtt" {
		data, err := os.ReadFile(tmp.Name())
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(abs, data, 0o644); err != nil {
			return nil, err
		}
	} else if err := s.convert(ctx, tmp.Name(), abs); err != nil {
		_, _ = s.Store.DeleteSubtitle(ctx, sub.ID)
		return nil, err
	}

	return s.Store.UpdateSubtitlePath(ctx, sub.ID, rel)
}

func (s *Service) convert(ctx context.Context, in, out string) error {
	cmdOut, err := exec.CommandContext(ctx, s.FFmpegPath,
		"-y", "-hide_banner", "-loglevel", "error",
		"-i", in, out).CombinedOutput()
	if err != nil {
		return fmt.Errorf("subtitle conversion: %s", strings.TrimSpace(string(cmdOut)))
	}
	return nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	sub, err := s.Store.DeleteSubtitle(ctx, id)
	if err != nil {
		return err
	}
	os.Remove(s.Path(sub))
	return nil
}

// ExtractPayload / Handle implement the extract_subtitles job: pull every
// embedded text stream out of a media file into side-car .vtt files.
type ExtractPayload struct {
	MediaFileID string `json:"mediaFileId"`
}

func (s *Service) HandleExtract(ctx context.Context, job *jobs.Job, report func(int)) error {
	var p ExtractPayload
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		return err
	}
	mf, err := s.Files.MediaFileByID(ctx, p.MediaFileID)
	if err != nil {
		return err
	}
	lib, err := s.Files.LibraryByID(ctx, mf.LibraryID)
	if err != nil {
		return err
	}
	abs := filepath.Join(lib.Path, mf.Path)

	var probe struct {
		Streams []struct {
			Index       int    `json:"index"`
			CodecType   string `json:"codec_type"`
			CodecName   string `json:"codec_name"`
			Disposition struct {
				Forced int `json:"forced"`
			} `json:"disposition"`
			Tags struct {
				Language string `json:"language"`
				Title    string `json:"title"`
			} `json:"tags"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(mf.Probe, &probe); err != nil {
		return fmt.Errorf("parse stored probe: %w", err)
	}

	if err := s.Store.DeleteEmbeddedSubtitles(ctx, mf.ID); err != nil {
		return err
	}

	extracted := 0
	for _, stream := range probe.Streams {
		if stream.CodecType != "subtitle" || !media.IsTextSubtitleCodec(stream.CodecName) {
			continue
		}
		lang := stream.Tags.Language
		if lang == "" {
			lang = "und"
		}
		label := stream.Tags.Title
		if label == "" {
			label = strings.ToUpper(lang)
		}

		sub, err := s.Store.CreateSubtitle(ctx, mf.ID, lang, label, "embedded",
			stream.Disposition.Forced == 1, "pending")
		if err != nil {
			return err
		}
		rel := filepath.Join("subtitles", mf.ID, sub.ID+".vtt")
		out := filepath.Join(s.DataDir, rel)
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}

		cmdOut, err := exec.CommandContext(ctx, s.FFmpegPath,
			"-y", "-hide_banner", "-loglevel", "error",
			"-i", abs,
			"-map", fmt.Sprintf("0:%d", stream.Index),
			"-f", "webvtt", out).CombinedOutput()
		if err != nil {
			// one bad stream shouldn't fail the rest
			_, _ = s.Store.DeleteSubtitle(ctx, sub.ID)
			slog.Warn("subtitle extraction failed", "file", mf.Path, "stream", stream.Index,
				"err", strings.TrimSpace(string(cmdOut)))
			continue
		}
		if _, err := s.Store.UpdateSubtitlePath(ctx, sub.ID, rel); err != nil {
			return err
		}
		extracted++
	}
	slog.Info("subtitles extracted", "file", mf.Path, "count", extracted)
	return nil
}
