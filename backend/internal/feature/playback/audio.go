package playback

import (
	"fmt"
	"strings"

	"couchverse/internal/media"
)

func audioLanguageCode(lang string) string {
	lang = strings.ToLower(strings.TrimSpace(lang))
	switch lang {
	case "eng":
		return "en"
	case "ces", "cze":
		return "cs"
	case "slk", "slo":
		return "sk"
	case "deu", "ger":
		return "de"
	case "fra", "fre":
		return "fr"
	case "spa":
		return "es"
	case "ita":
		return "it"
	case "pol":
		return "pl"
	case "rus":
		return "ru"
	case "por":
		return "pt"
	case "nld", "dut":
		return "nl"
	case "hun":
		return "hu"
	case "jpn":
		return "ja"
	case "kor":
		return "ko"
	case "zho", "chi":
		return "zh"
	}
	return lang
}

type audioFileSource struct {
	file            media.MediaFile
	streams         []media.AudioStream
	multiAudioReady bool
}

// A title can contain both embedded audio and separate-language files. Each
// embedded choice carries its own playlist so switching back from a sibling
// cannot accidentally select audio on the sibling's player source.
func audioTracksForSources(sources []audioFileSource) []audioTrack {
	tracks := []audioTrack{}
	for sourceIndex, source := range sources {
		primary := sourceIndex == 0
		if len(source.streams) < 2 {
			if len(sources) > 1 {
				tracks = append(tracks, audioTrackFor(&source.file, primary))
			}
			continue
		}
		if !source.multiAudioReady {
			continue
		}
		defaultIndex := 0
		for i, stream := range source.streams {
			if stream.Default {
				defaultIndex = i
				break
			}
		}
		for i, stream := range source.streams {
			label := stream.Title
			if label == "" || label == stream.Lang {
				label = audioLabel(stream.Lang)
			}
			tracks = append(tracks, audioTrack{
				ID:            fmt.Sprintf("embedded:%s:%d", source.file.ID, stream.Index),
				Lang:          audioLanguageCode(stream.Lang),
				Label:         label,
				Default:       primary && i == defaultIndex,
				Source:        "embedded",
				HLSURL:        "/api/v1/stream/" + source.file.ID + "/hls/multiaudio/master.m3u8",
				HLSAudioIndex: &i,
			})
		}
	}
	return tracks
}
