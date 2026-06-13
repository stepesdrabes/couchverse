// Package artwork stores poster/backdrop/cover images, produces resized
// variants on demand by shelling out to ffmpeg, and extracts a vibrant accent
// colour per image (stdlib decode, see accent.go) for the UI to theme with.
package artwork

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// accentUnknown marks an artwork whose image yielded no colour (or couldn't be
// decoded), so the backfill records it as processed instead of retrying it. The
// UI ignores any accent that isn't a valid hex.
const accentUnknown = "-"

// AccentFor extracts an artwork's accent, returning the sentinel when none was
// found so the value is never left empty. Used at every write site (uploads,
// TMDB downloads, embedded covers) so callers store a stable accent in one step.
func AccentFor(path string) string {
	if hex := ExtractAccent(path); hex != "" {
		return hex
	}
	return accentUnknown
}

type Service struct {
	Store      *Store
	DataDir    string
	FFmpegPath string
}

var sizes = map[string]int{
	"w342": 342,
	"w780": 780,
}

var allowedExts = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true}

// Save stores an uploaded original and upserts the artwork slot.
func (s *Service) Save(ctx context.Context, ownerKind string, ownerID string, kind, filename string, body io.Reader) (*Artwork, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	if !allowedExts[ext] {
		return nil, fmt.Errorf("unsupported image type %q (jpg/png/webp)", ext)
	}

	rel := filepath.Join("artwork", ownerKind, ownerID, kind+ext)
	abs := filepath.Join(s.DataDir, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return nil, err
	}
	f, err := os.Create(abs)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(f, io.LimitReader(body, 30<<20)); err != nil {
		f.Close()
		return nil, err
	}
	f.Close()

	art, err := s.Store.SetArtwork(ctx, ownerKind, ownerID, kind, rel, 0, 0, "uploaded", AccentFor(abs))
	if err != nil {
		return nil, err
	}
	s.dropCache(art.ID)
	return art, nil
}

// SaveBytes is used by metadata jobs (TMDB downloads, embedded covers).
func (s *Service) SaveBytes(ctx context.Context, ownerKind string, ownerID string, kind, ext string, data []byte, source string) (*Artwork, error) {
	rel := filepath.Join("artwork", ownerKind, ownerID, kind+ext)
	abs := filepath.Join(s.DataDir, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(abs, data, 0o644); err != nil {
		return nil, err
	}
	art, err := s.Store.SetArtwork(ctx, ownerKind, ownerID, kind, rel, 0, 0, source, AccentFor(abs))
	if err != nil {
		return nil, err
	}
	s.dropCache(art.ID)
	return art, nil
}

// Resolve returns the on-disk path for an artwork id at the requested size,
// generating and caching the resized variant on first use. The cache name is
// derived from the original's mtime, so a replaced image (TMDB re-apply,
// database reset reusing ids) can never serve a stale resize.
func (s *Service) Resolve(ctx context.Context, art *Artwork, size string) (string, error) {
	original := filepath.Join(s.DataDir, art.Path)
	width, ok := sizes[size]
	if !ok {
		return original, nil
	}

	info, err := os.Stat(original)
	if err != nil {
		return "", err
	}
	cached := filepath.Join(s.DataDir, "cache", "images",
		fmt.Sprintf("%s_%d_%s.jpg", art.ID, info.ModTime().UnixNano(), size))
	if _, err := os.Stat(cached); err == nil {
		return cached, nil
	}
	if err := os.MkdirAll(filepath.Dir(cached), 0o755); err != nil {
		return "", err
	}
	// drop resizes of older versions of this artwork
	if stale, err := filepath.Glob(filepath.Join(s.DataDir, "cache", "images",
		fmt.Sprintf("%s_*_%s.jpg", art.ID, size))); err == nil {
		for _, f := range stale {
			os.Remove(f)
		}
	}

	out, err := exec.CommandContext(ctx, s.FFmpegPath,
		"-y", "-hide_banner", "-loglevel", "error",
		"-i", original,
		"-vf", fmt.Sprintf("scale=%d:-2", width),
		"-frames:v", "1", "-update", "1",
		cached).CombinedOutput()
	if err != nil {
		os.Remove(cached)
		return "", fmt.Errorf("resize: %s", strings.TrimSpace(string(out)))
	}
	return cached, nil
}

// Delete removes the artwork row, original file and cached sizes.
func (s *Service) Delete(ctx context.Context, id string) error {
	art, err := s.Store.DeleteArtwork(ctx, id)
	if err != nil {
		return err
	}
	os.Remove(filepath.Join(s.DataDir, art.Path))
	s.dropCache(art.ID)
	return nil
}

func (s *Service) dropCache(id string) {
	stale, err := filepath.Glob(filepath.Join(s.DataDir, "cache", "images", id+"_*"))
	if err != nil {
		return
	}
	for _, f := range stale {
		os.Remove(f)
	}
}

// BackfillAccents fills the accent for artwork rows that predate accent
// extraction (existing libraries). Meant to run once in a background goroutine
// on startup: each row is marked when processed, so it never loops and a
// restart resumes where it left off. Gentle on CPU so it doesn't fight startup.
func (s *Service) BackfillAccents(ctx context.Context) {
	const batch = 200
	total := 0
	for {
		rows, err := s.Store.ArtworkMissingAccent(ctx, batch)
		if err != nil {
			slog.Warn("accent backfill query", "err", err)
			return
		}
		if len(rows) == 0 {
			if total > 0 {
				slog.Info("artwork accent backfill done", "processed", total)
			}
			return
		}
		for _, art := range rows {
			if ctx.Err() != nil {
				return
			}
			if err := s.Store.SetAccent(ctx, art.ID, AccentFor(filepath.Join(s.DataDir, art.Path))); err != nil {
				slog.Warn("accent backfill", "id", art.ID, "err", err)
			}
			total++
			time.Sleep(20 * time.Millisecond)
		}
	}
}

// DeleteForOwner removes all artwork rows, files and cached resizes of an
// owner - called when a title or album is deleted.
func (s *Service) DeleteForOwner(ctx context.Context, ownerKind string, ownerID string) error {
	rows, err := s.Store.DeleteArtworkForOwner(ctx, ownerKind, ownerID)
	if err != nil {
		return err
	}
	for _, art := range rows {
		os.Remove(filepath.Join(s.DataDir, art.Path))
		s.dropCache(art.ID)
	}
	os.Remove(filepath.Join(s.DataDir, "artwork", ownerKind, ownerID))
	return nil
}
