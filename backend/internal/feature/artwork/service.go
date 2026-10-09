// Package artwork stores poster/backdrop/thumb/logo images, produces resized
// variants on demand by shelling out to ffmpeg, and extracts a vibrant accent
// colour per image (stdlib decode, see accent.go) for the UI to theme with.
package artwork

import (
	"context"
	"fmt"
	"image"
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
// TMDB downloads) so callers store a stable accent in one step.
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
func (s *Service) Save(ctx context.Context, ownerKind string, ownerID string, kind, lang, filename string, body io.Reader) (*Artwork, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	if !allowedExts[ext] {
		return nil, fmt.Errorf("unsupported image type %q (jpg/png/webp)", ext)
	}
	if kind == "logo" && ext != ".png" {
		return nil, fmt.Errorf("a logo must be a .png image, which keeps its transparency")
	}

	rel := slotPath(ownerKind, ownerID, kind, lang, ext)
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
	return s.record(ctx, ownerKind, ownerID, kind, lang, rel, "uploaded")
}

// SaveBytes is used by metadata jobs (TMDB downloads).
func (s *Service) SaveBytes(ctx context.Context, ownerKind string, ownerID string, kind, lang, ext string, data []byte, source string) (*Artwork, error) {
	rel := slotPath(ownerKind, ownerID, kind, lang, ext)
	abs := filepath.Join(s.DataDir, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(abs, data, 0o644); err != nil {
		return nil, err
	}
	return s.record(ctx, ownerKind, ownerID, kind, lang, rel, source)
}

// slotPath names a slot's original file; a language-bound slot (a logo) gets
// the language in its name so the slots of one owner never share a file.
func slotPath(ownerKind, ownerID, kind, lang, ext string) string {
	name := kind
	if lang != "" {
		name += "-" + lang
	}
	return filepath.Join("artwork", ownerKind, ownerID, name+ext)
}

// record upserts the slot for a freshly written original, measuring it and
// extracting its accent.
func (s *Service) record(ctx context.Context, ownerKind, ownerID, kind, lang, rel, source string) (*Artwork, error) {
	abs := filepath.Join(s.DataDir, rel)
	if kind == "logo" {
		// a logo that cannot be trimmed is still a logo; it keeps its padding
		if err := trimTransparentMargins(abs); err != nil {
			slog.Warn("trim logo", "path", rel, "err", err)
		}
	}
	w, h := dimensions(abs)
	art, err := s.Store.SetArtwork(ctx, ownerKind, ownerID, kind, lang, rel, w, h, source, AccentFor(abs))
	if err != nil {
		return nil, err
	}
	s.dropCache(art.ID)
	return art, nil
}

// dimensions reads an image's size from its header; 0x0 for a format the
// standard library cannot decode (WebP).
func dimensions(path string) (width, height int) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0
	}
	return cfg.Width, cfg.Height
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
	// a logo is drawn over other art, so its resize has to keep the alpha
	// channel that JPEG would flatten
	ext := ".jpg"
	if art.Kind == "logo" {
		ext = ".png"
	}

	info, err := os.Stat(original)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(s.DataDir, "cache", "images")
	cached := filepath.Join(dir, fmt.Sprintf("%s_%d_%s%s", art.ID, info.ModTime().UnixNano(), size, ext))
	if _, err := os.Stat(cached); err == nil {
		return cached, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	// drop resizes of older versions of this artwork (not this one: a request for the same
	// image may just have made it)
	if stale, err := filepath.Glob(filepath.Join(dir, fmt.Sprintf("%s_*_%s%s", art.ID, size, ext))); err == nil {
		for _, f := range stale {
			if f != cached {
				os.Remove(f)
			}
		}
	}

	// resized aside and moved into place, so a request at the same moment never serves a
	// half-written image
	tmp, err := os.CreateTemp(dir, ".resize-*"+ext)
	if err != nil {
		return "", err
	}
	tmp.Close()
	defer os.Remove(tmp.Name())
	out, err := exec.CommandContext(ctx, s.FFmpegPath,
		"-y", "-hide_banner", "-loglevel", "error",
		"-i", original,
		"-vf", fmt.Sprintf("scale=%d:-2", width),
		"-frames:v", "1", "-update", "1",
		tmp.Name()).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("resize: %s", strings.TrimSpace(string(out)))
	}
	if err := os.Rename(tmp.Name(), cached); err != nil {
		return "", err
	}
	return cached, nil
}

// Delete removes the artwork row, original file and cached sizes.
func (s *Service) Delete(ctx context.Context, id string) error {
	art, err := s.Store.DeleteArtwork(ctx, id)
	if err != nil {
		return err
	}
	s.removeFiles(*art)
	return nil
}

// DeleteForLang removes an owner's artwork in one language with its files -
// called when a title drops a content language.
func (s *Service) DeleteForLang(ctx context.Context, ownerKind string, ownerID string, lang string) error {
	rows, err := s.Store.DeleteArtworkForLang(ctx, ownerKind, ownerID, lang)
	if err != nil {
		return err
	}
	for _, art := range rows {
		s.removeFiles(art)
	}
	return nil
}

func (s *Service) removeFiles(art Artwork) {
	os.Remove(filepath.Join(s.DataDir, art.Path))
	s.dropCache(art.ID)
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
// owner - called when a title is deleted.
func (s *Service) DeleteForOwner(ctx context.Context, ownerKind string, ownerID string) error {
	rows, err := s.Store.DeleteArtworkForOwner(ctx, ownerKind, ownerID)
	if err != nil {
		return err
	}
	for _, art := range rows {
		s.removeFiles(art)
	}
	os.Remove(filepath.Join(s.DataDir, "artwork", ownerKind, ownerID))
	return nil
}
