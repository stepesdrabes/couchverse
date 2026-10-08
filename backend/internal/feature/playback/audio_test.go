package playback

import (
	"testing"
	"time"

	"couchverse/internal/media"
)

func TestAudioTracksCombineEmbeddedPrimaryAndSeparateLanguages(t *testing.T) {
	tracks := audioTracksForSources([]audioFileSource{
		{
			file: media.MediaFile{ID: "combined", DirectPlay: true, AudioLang: "en"},
			streams: []media.AudioStream{
				{Index: 2, Lang: "eng"},
				{Index: 5, Lang: "ces", Default: true},
			},
			multiAudioReady: true,
		},
		{file: media.MediaFile{ID: "german", DirectPlay: true, AudioLang: "de", AudioRole: "audio_alt"}},
	})
	if len(tracks) != 3 {
		t.Fatalf("got %d tracks, want English, Czech and German", len(tracks))
	}
	for i, want := range []string{"en", "cs"} {
		track := tracks[i]
		if track.Source != "embedded" || track.Lang != want {
			t.Errorf("track %d = %+v, want embedded %s", i, track, want)
		}
		if track.HLSURL != "/api/v1/stream/combined/hls/multiaudio/master.m3u8" || track.StreamURL != "" {
			t.Errorf("combined track must use its multiaudio playlist: %+v", track)
		}
		if track.HLSAudioIndex == nil || *track.HLSAudioIndex != i {
			t.Errorf("track %d must use output audio index %d, not its source index: %+v", i, i, track)
		}
	}
	if tracks[0].Default || !tracks[1].Default || tracks[2].Default {
		t.Errorf("only the primary file's Czech default should be selected: %+v", tracks)
	}
	if tracks[0].Label != "English" || tracks[1].Label != "Čeština" {
		t.Errorf("ISO 639-2 container tags must have readable labels: %+v", tracks)
	}
	if tracks[2].Source != "file" || tracks[2].StreamURL != "/api/v1/stream/german" {
		t.Errorf("separate German file must retain its direct URL: %+v", tracks[2])
	}
}

func TestCombinedSiblingUsesItsOwnHLSAndDistinctTrackIDs(t *testing.T) {
	streams := []media.AudioStream{{Index: 1, Lang: "eng", Default: true}, {Index: 3, Lang: "ces"}}
	tracks := audioTracksForSources([]audioFileSource{
		{file: media.MediaFile{ID: "primary", DirectPlay: true}, streams: streams, multiAudioReady: true},
		{file: media.MediaFile{ID: "sibling", DirectPlay: true}, streams: streams, multiAudioReady: true},
	})
	if len(tracks) != 4 {
		t.Fatalf("got %d tracks, want both tracks from both files", len(tracks))
	}
	seen := map[string]bool{}
	for i, track := range tracks {
		if seen[track.ID] {
			t.Errorf("track IDs collide for identical source stream indexes: %s", track.ID)
		}
		seen[track.ID] = true
		if track.Default != (i == 0) {
			t.Errorf("only the primary's default should be selected: %+v", track)
		}
		if i >= 2 && (track.HLSURL != "/api/v1/stream/sibling/hls/multiaudio/master.m3u8" || track.StreamURL != "") {
			t.Errorf("combined sibling must use its own multiaudio HLS: %+v", track)
		}
	}
}

func TestAudioMenuWaitsForCombinedSiblingPreparation(t *testing.T) {
	tracks := audioTracksForSources([]audioFileSource{
		{file: media.MediaFile{ID: "primary", AudioLang: "en", DirectPlay: true}},
		{file: media.MediaFile{ID: "preparing", DirectPlay: true},
			streams: []media.AudioStream{{Lang: "eng"}, {Lang: "ces"}}},
	})
	if len(tracks) != 1 || tracks[0].ID != "primary" || !tracks[0].Default {
		t.Errorf("unprepared combined file must not offer a broken direct/HLS choice: %+v", tracks)
	}
}

func TestEmbeddedDefaultIsDeterministic(t *testing.T) {
	for _, flagged := range []bool{false, true} {
		tracks := audioTracksForSources([]audioFileSource{{
			file: media.MediaFile{ID: "primary"}, multiAudioReady: true,
			streams: []media.AudioStream{{Index: 1, Default: flagged}, {Index: 2, Default: flagged}},
		}})
		if len(tracks) != 2 || !tracks[0].Default || tracks[1].Default {
			t.Errorf("zero or multiple input defaults must select exactly the first track: %+v", tracks)
		}
	}
}

func TestSingleAudioPlaybackKeepsExistingBehavior(t *testing.T) {
	if tracks := audioTracksForSources([]audioFileSource{{file: media.MediaFile{ID: "single"}}}); len(tracks) != 0 {
		t.Errorf("one single-audio file does not need a language menu: %+v", tracks)
	}
	deleted := time.Unix(0, 0)
	tracks := audioTracksForSources([]audioFileSource{
		{file: media.MediaFile{ID: "primary", DirectPlay: true, AudioLang: "eng"}},
		{file: media.MediaFile{ID: "alternate", DirectPlay: true, AudioLang: "cze", SourceDeletedAt: &deleted}},
	})
	if len(tracks) != 2 || tracks[0].Lang != "en" || tracks[1].Lang != "cs" {
		t.Fatalf("single-file language aliases must be normalized: %+v", tracks)
	}
	if tracks[1].StreamURL != "" || tracks[1].HLSURL != "/api/v1/stream/alternate/hls/master.m3u8" {
		t.Errorf("deleted source must use prepared HLS: %+v", tracks[1])
	}
}

func TestAudioLanguageLabels(t *testing.T) {
	for lang, want := range map[string]string{"eng": "English", "en": "English", "ces": "Čeština", "cze": "Čeština", "cs": "Čeština", " ENG ": "English", "und": "Original"} {
		if got := audioLabel(lang); got != want {
			t.Errorf("audioLabel(%q) = %q, want %q", lang, got, want)
		}
	}
}

func TestSelectorLanguageAliases(t *testing.T) {
	aliases := map[string]string{
		"eng": "en", "ces": "cs", "cze": "cs", "slk": "sk", "slo": "sk",
		"deu": "de", "ger": "de", "fra": "fr", "fre": "fr", "spa": "es",
		"ita": "it", "pol": "pl", "rus": "ru", "por": "pt", "nld": "nl",
		"dut": "nl", "hun": "hu", "jpn": "ja", "kor": "ko", "zho": "zh", "chi": "zh",
	}
	for alias, want := range aliases {
		if got := audioLanguageCode(alias); got != want {
			t.Errorf("audioLanguageCode(%q) = %q, want %q", alias, got, want)
		}
		if got := audioLabel(alias); got != audioLabel(want) {
			t.Errorf("audioLabel(%q) = %q, want %q", alias, got, audioLabel(want))
		}
	}
	for _, code := range []string{"en", "cs", "sk", "de", "fr", "es", "it", "pl", "ru", "pt", "nl", "hu", "ja", "ko", "zh", "und", "unknown"} {
		if got := audioLanguageCode(code); got != code {
			t.Errorf("audioLanguageCode(%q) changed an existing tag to %q", code, got)
		}
	}
}
