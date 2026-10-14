package couchtv

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// A file with no subtitles gives no scenes, and nothing is decoded to look: what is said in a scene
// nobody read is unknown, and a trailer is not to carry it. (The ffmpeg path names nothing, so a
// decode would fail the pick.)
func TestPickNeedsSubtitles(t *testing.T) {
	p := &scenePicker{FFmpeg: filepath.Join(t.TempDir(), "no-ffmpeg"), Risk: SceneViolence}
	file := pickFile{ID: "a", Path: "a.mkv", Duration: 6000, Audio: dialogueTrack{FileID: "a", Lang: "en"}}
	got, err := p.pickTitle(context.Background(), []pickFile{file})
	if err != nil || len(got) != 0 {
		t.Errorf("a file without subtitles: %d scenes, %v", len(got), err)
	}

	// a subtitle file that cannot be read is no subtitles
	file.Subs = []string{filepath.Join(t.TempDir(), "gone.vtt")}
	if got, err := p.pickTitle(context.Background(), []pickFile{file}); err != nil || len(got) != 0 {
		t.Errorf("a file whose subtitles are gone: %d scenes, %v", len(got), err)
	}

	// subtitles with nothing to say are no scenes either
	empty := filepath.Join(t.TempDir(), "empty.vtt")
	if err := os.WriteFile(empty, []byte("WEBVTT\n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	file.Subs = []string{empty}
	if got, err := p.pickTitle(context.Background(), []pickFile{file}); err != nil || len(got) != 0 {
		t.Errorf("a file with an empty subtitle file: %d scenes, %v", len(got), err)
	}
}

// The key of a title's subtitles does not depend on the order of its files or tracks, and changes
// when one is added, taken away or replaced.
func TestSubtitleKey(t *testing.T) {
	a := pickFile{SubPaths: []string{"subtitles/f1/en.vtt", "subtitles/f1/sdh.vtt"}}
	b := pickFile{SubPaths: []string{"subtitles/f2/en.vtt"}}
	key := subtitleKey([]pickFile{a, b})
	if swapped := subtitleKey([]pickFile{b, {SubPaths: []string{"subtitles/f1/sdh.vtt", "subtitles/f1/en.vtt"}}}); swapped != key {
		t.Errorf("the key follows the order: %q and %q", key, swapped)
	}
	for name, other := range map[string][]pickFile{
		"one less":  {a},
		"one more":  {a, b, {SubPaths: []string{"subtitles/f3/en.vtt"}}},
		"replaced":  {a, {SubPaths: []string{"subtitles/f2/new.vtt"}}},
		"no tracks": {{}, {}},
	} {
		if got := subtitleKey(other); got == key {
			t.Errorf("%s: the key is the same: %q", name, got)
		}
	}
	if subtitleKey(nil) != "" || subtitleKey([]pickFile{{}}) != "" {
		t.Error("no subtitles have a key")
	}
}
