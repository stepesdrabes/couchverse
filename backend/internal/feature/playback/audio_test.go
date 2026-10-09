package playback

import (
	"fmt"
	"testing"

	"couchverse/internal/media"
)

func TestAudioMenuJoinsEmbeddedTracksAndSiblings(t *testing.T) {
	menu := audioMenu([]audioFile{
		{file: &media.MediaFile{ID: "combined", AudioLang: "en"},
			tracks: []media.AudioStream{{Index: 2, Lang: "eng"}, {Index: 5, Lang: "ces", Default: true}}},
		{file: &media.MediaFile{ID: "german", AudioLang: "de", AudioRole: "audio_alt"}, mode: "direct", url: "/german/stream"},
	})
	if len(menu) != 3 {
		t.Fatalf("got %+v, want English, Czech and German", menu)
	}
	for i, want := range []struct{ id, lang, label string }{
		{"embedded:combined:2", "en", "English"},
		{"embedded:combined:5", "cs", "Čeština"},
	} {
		got := menu[i]
		if got.ID != want.id || got.Lang != want.lang || got.Label != want.label || got.Source != "embedded" {
			t.Errorf("track %d = %+v, want %+v", i, got, want)
		}
		if got.StreamURL != "" || got.HLSURL != "" {
			t.Errorf("track %d: the file's own tracks play from its source: %+v", i, got)
		}
	}
	if menu[0].Default || !menu[1].Default || menu[2].Default {
		t.Errorf("only the file's own default plays without a choice: %+v", menu)
	}
	german := menu[2]
	if german.ID != "german" || german.Source != "file" || german.Lang != "de" || german.StreamURL != "/german/stream" {
		t.Errorf("the German sibling plays its own file: %+v", german)
	}
}

func TestSiblingTracksCarryTheirFile(t *testing.T) {
	tracks := []media.AudioStream{{Index: 1, Lang: "eng", Default: true}, {Index: 3, Lang: "ces"}}
	menu := audioMenu([]audioFile{
		{file: &media.MediaFile{ID: "primary"}, tracks: tracks},
		{file: &media.MediaFile{ID: "sibling"}, tracks: tracks, mode: "hls", url: "/sibling/master.m3u8"},
	})
	if len(menu) != 4 {
		t.Fatalf("got %+v, want both tracks of both files", menu)
	}
	seen := map[string]bool{}
	for i, track := range menu {
		if seen[track.ID] {
			t.Errorf("ids repeat across files: %s", track.ID)
		}
		seen[track.ID] = true
		if track.Default != (i == 0) {
			t.Errorf("track %d: only the file's own default plays without a choice: %+v", i, track)
		}
		if sibling := i >= 2; sibling != (track.HLSURL == "/sibling/master.m3u8") || track.StreamURL != "" {
			t.Errorf("track %d: a sibling's tracks name its playlist, the file's own nothing: %+v", i, track)
		}
	}
}

func TestAudioMenuHoldsBackTracksThatCannotSwitch(t *testing.T) {
	two := []media.AudioStream{{Index: 1, Lang: "eng", Default: true}, {Index: 2, Lang: "ces"}}
	// a sibling whose tracks this device cannot switch yet is one choice, in
	// the language it plays
	menu := audioMenu([]audioFile{
		{file: &media.MediaFile{ID: "primary", AudioLang: "de"}},
		{file: &media.MediaFile{ID: "preparing"}, tracks: two, mode: "direct", url: "/preparing/stream", defaultAudioOnly: true},
	})
	if len(menu) != 2 || menu[1].ID != "preparing" || menu[1].Source != "file" || menu[1].Lang != "en" ||
		menu[1].StreamURL != "/preparing/stream" {
		t.Errorf("an unprepared sibling must not offer tracks it cannot switch to: %+v", menu)
	}
	// the file's own stays reachable beside a sibling
	menu = audioMenu([]audioFile{
		{file: &media.MediaFile{ID: "primary", AudioLang: "cs"}, tracks: two, defaultAudioOnly: true},
		{file: &media.MediaFile{ID: "german", AudioLang: "de"}, mode: "hls", url: "/german/master.m3u8"},
	})
	if len(menu) != 2 || menu[0].ID != "primary" || menu[0].Lang != "cs" || !menu[0].Default || menu[0].StreamURL != "" {
		t.Errorf("the file's own must stay one choice: %+v", menu)
	}
	// and alone it is no choice at all
	if menu := audioMenu([]audioFile{{file: &media.MediaFile{ID: "primary"}, tracks: two, defaultAudioOnly: true}}); menu != nil {
		t.Errorf("one track that plays needs no menu: %+v", menu)
	}
}

func TestAudioMenuLeavesOutSiblingsThatCannotPlay(t *testing.T) {
	for _, mode := range []string{"preparing", "unsupported"} {
		menu := audioMenu([]audioFile{
			{file: &media.MediaFile{ID: "primary", AudioLang: "en"}},
			{file: &media.MediaFile{ID: "sibling", AudioLang: "cs"}, mode: mode},
		})
		if menu != nil {
			t.Errorf("%s: a sibling without a source would play the file's own: %+v", mode, menu)
		}
	}
}

func TestEmbeddedDefaultIsDeterministic(t *testing.T) {
	for _, flagged := range []bool{false, true} {
		menu := audioMenu([]audioFile{{file: &media.MediaFile{ID: "primary"},
			tracks: []media.AudioStream{{Index: 1, Default: flagged}, {Index: 2, Default: flagged}}}})
		if len(menu) != 2 || !menu[0].Default || menu[1].Default {
			t.Errorf("defaults %v: exactly the first track must be the default: %+v", flagged, menu)
		}
	}
}

func TestFileLanguagesAreNormalized(t *testing.T) {
	if menu := audioMenu([]audioFile{{file: &media.MediaFile{ID: "single"}}}); menu != nil {
		t.Errorf("one file in one language needs no menu: %+v", menu)
	}
	menu := audioMenu([]audioFile{
		{file: &media.MediaFile{ID: "primary", AudioLang: "eng"}},
		{file: &media.MediaFile{ID: "alternate", AudioLang: "cze"}, mode: "hls", url: "/alternate/master.m3u8"},
		{file: &media.MediaFile{ID: "untagged"}, tracks: []media.AudioStream{{Index: 1, Lang: "ger"}}, mode: "direct", url: "/untagged/stream"},
	})
	got := []string{}
	for _, track := range menu {
		got = append(got, track.Lang+" "+track.Label)
	}
	if fmt.Sprint(got) != "[en English cs Čeština de Deutsch]" {
		t.Errorf("languages %v", got)
	}
	if menu[1].StreamURL != "" || menu[1].HLSURL != "/alternate/master.m3u8" {
		t.Errorf("a sibling that does not direct-play gives its playlist: %+v", menu[1])
	}
}

func TestAudioLanguageLabels(t *testing.T) {
	for lang, want := range map[string]string{
		"eng": "English", "en": "English", " ENG ": "English", "ces": "Čeština", "cze": "Čeština", "cs": "Čeština",
		"und": "Original", "": "Original", "unknown": "Original", "swe": "SWE",
	} {
		if got := audioLabel(lang); got != want {
			t.Errorf("audioLabel(%q) = %q, want %q", lang, got, want)
		}
	}
	for alias, code := range map[string]string{
		"slk": "sk", "slo": "sk", "deu": "de", "ger": "de", "fra": "fr", "fre": "fr", "spa": "es", "ita": "it",
		"pol": "pl", "rus": "ru", "por": "pt", "nld": "nl", "dut": "nl", "hun": "hu", "jpn": "ja", "kor": "ko",
		"zho": "zh", "chi": "zh",
	} {
		if got := media.BCP47(alias); got != code {
			t.Errorf("BCP47(%q) = %q, want %q", alias, got, code)
		}
		if got, want := audioLabel(alias), audioLangNames[code]; got != want {
			t.Errorf("audioLabel(%q) = %q, want %q", alias, got, want)
		}
	}
}
