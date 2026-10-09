package server_test

import (
	"testing"

	"couchverse/internal/feature/catalog"
)

// TestAdminAudioInventory checks the audio tracks the title editor reads per file:
// a probed file's in stream order, and no entry at all for a file never probed,
// whose languages an uploader must treat as unknown.
func TestAdminAudioInventory(t *testing.T) {
	env := newTestEnv(t)

	var series catalog.AdminTitle
	env.getJSON(t, "admin", "/admin/titles/"+seriesID, &series)
	tracks, ok := series.AudioStreamsByFile[hdrFileID]
	if !ok || len(tracks) != 2 {
		t.Fatalf("HDR file tracks %+v, want two", series.AudioStreamsByFile)
	}
	first, second := tracks[0], tracks[1]
	if first.Index != 1 || first.Codec != "eac3" || first.Lang != "en" || first.Title != "English 5.1" ||
		first.Channels != 6 || !first.Default {
		t.Errorf("first track %+v", first)
	}
	if second.Index != 2 || second.Lang != "cs" || second.Default {
		t.Errorf("second track %+v", second)
	}
	if _, ok := series.AudioStreamsByFile[ladderFileID]; ok {
		t.Errorf("a file without probed tracks has an entry: %+v", series.AudioStreamsByFile[ladderFileID])
	}

	var movie catalog.AdminTitle
	env.getJSON(t, "admin", "/admin/titles/"+movieID, &movie)
	if len(movie.MediaFiles) != 2 || movie.AudioStreamsByFile == nil || len(movie.AudioStreamsByFile) != 0 {
		t.Errorf("movie files %d, tracks %+v, want two files and no entries", len(movie.MediaFiles), movie.AudioStreamsByFile)
	}
}
