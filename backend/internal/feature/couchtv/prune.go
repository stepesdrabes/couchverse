package couchtv

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// How long rendered media is kept (the hourly cleanup prunes it).
const (
	// airedKeep keeps a break's package a little after it aired; its row (the recipe the
	// next breaks' promos read) is kept for breakKeep.
	airedKeep = time.Hour
	breakKeep = 7 * 24 * time.Hour
	// clipKeep drops a clip no composition has asked for in that long.
	clipKeep = 14 * 24 * time.Hour
	// extractKeep drops extracted frames and excerpts no render has used in that long.
	extractKeep = 14 * 24 * time.Hour
	// soundKeep drops an item's sound no break has mixed in that long: most are one break's
	// (a squeeze, an up next), assembled within a day of their render; the shared ones (idents,
	// stings, promos) are mixed every day, and one dropped too soon is rendered again.
	soundKeep = 2 * 24 * time.Hour
	// workKeep drops what a crashed render left in its work directory.
	workKeep = 24 * time.Hour
)

// Prune keeps the rendered media bounded: the packages of breaks that have aired, clips
// no break has asked for in a while (never one a waiting break uses), evergreen breaks
// older than the newest assembled one, and extracted media, items' sounds, work
// directories and soundtrack previews nobody used lately (never a title's pick); and it
// forgets guests long gone. It runs in the hourly cleanup.
func Prune(ctx context.Context, s *Store, dataDir string, now time.Time) error {
	remove := func(query string, args ...any) (int, error) {
		rows, err := s.q.Query(ctx, query, args...)
		if err != nil {
			return 0, err
		}
		defer rows.Close()
		n := 0
		for rows.Next() {
			var path string
			if err := rows.Scan(&path); err != nil {
				return n, err
			}
			if path != "" {
				os.RemoveAll(filepath.Join(dataDir, path))
			}
			n++
		}
		return n, rows.Err()
	}

	aired, err := remove(`
		UPDATE couchtv_breaks b SET path = '', size_bytes = 0 FROM couchtv_airings a, couchtv_breaks old
		WHERE a.id = b.airing_id AND old.id = b.id AND b.path <> '' AND a.ends_at < $1
		RETURNING old.path`, now.Add(-airedKeep))
	if err != nil {
		return err
	}
	if _, err := s.q.Exec(ctx, `DELETE FROM couchtv_breaks b USING couchtv_airings a
		WHERE a.id = b.airing_id AND a.ends_at < $1`, now.Add(-breakKeep)); err != nil {
		return err
	}
	evergreen, err := remove(`
		DELETE FROM couchtv_breaks b WHERE b.kind = 'evergreen' AND b.created_at < (
			SELECT max(created_at) FROM couchtv_breaks WHERE kind = 'evergreen' AND state = 'ready')
		RETURNING b.path`)
	if err != nil {
		return err
	}
	clips, err := remove(`
		DELETE FROM couchtv_clips c WHERE c.last_used_at < $1 AND NOT EXISTS (
			SELECT 1 FROM couchtv_breaks b WHERE b.state IN ('composed', 'rendering')
			AND b.recipe->'clips' @> jsonb_build_array(jsonb_build_object('clipId', c.id::text)))
		RETURNING c.path`, now.Add(-clipKeep))
	if err != nil {
		return err
	}
	if aired+evergreen+clips > 0 {
		slog.Info("cleanup: pruned couch tv renders", "airedBreaks", aired, "evergreenBreaks", evergreen, "clips", clips)
	}
	// a guest who has not tuned in for as long as their cookie lasts is gone
	if _, err := s.q.Exec(ctx, `DELETE FROM couchtv_guests WHERE seen_at < $1`, now.Add(-guestKeep)); err != nil {
		return err
	}

	root := filepath.Join(dataDir, "couchtv")
	for _, dir := range []struct {
		pattern string
		keep    time.Duration
	}{
		{filepath.Join(root, "frames", "*", "*"), extractKeep},
		{filepath.Join(root, "audio", "*", "*"), extractKeep},
		{filepath.Join(root, soundsDir, "*"), soundKeep},
		{filepath.Join(root, "tmp", "*"), workKeep},
	} {
		matches, _ := filepath.Glob(dir.pattern)
		for _, m := range matches {
			if info, err := os.Stat(m); err == nil && now.Sub(info.ModTime()) > dir.keep {
				os.RemoveAll(m)
			}
		}
	}
	// previews nobody picked, which the admin listened to a while ago
	picked, err := s.soundtrackPaths(ctx)
	if err != nil {
		return err
	}
	previews, _ := filepath.Glob(filepath.Join(root, "soundtracks", "*"))
	for _, p := range previews {
		rel, _ := filepath.Rel(dataDir, p)
		if info, err := os.Stat(p); err == nil && !picked[rel] && now.Sub(info.ModTime()) > extractKeep {
			os.Remove(p)
		}
	}
	return nil
}
