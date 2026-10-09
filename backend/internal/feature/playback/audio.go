package playback

import (
	"fmt"

	"couchverse/internal/media"
)

// audioFile is one of a movie's or an episode's media files as its audio menu
// sees it: the payload's own file first, then the separate-language siblings
// (model B).
type audioFile struct {
	file   *media.MediaFile
	tracks []media.AudioStream
	// mode and url say how a sibling plays for the device (its own decision);
	// the payload's file plays from the payload's source at whichever quality
	mode, url string
	// defaultAudioOnly is the file's Decision.DefaultAudioOnly
	defaultAudioOnly bool
}

// audioMenu lists the audio choices across the files: a file's embedded tracks
// (model A) where its player can switch between them, else the file itself in
// the language it plays. Only the payload's file has a default, and a sibling
// that cannot play on the device is left out. It is empty with one choice.
func audioMenu(files []audioFile) []PlaybackAudioTrack {
	menu := []PlaybackAudioTrack{}
	for i, f := range files {
		own := i == 0
		var stream, hls string
		switch {
		case own:
		case f.mode == "direct":
			stream = f.url
		case f.mode == "hls":
			hls = f.url
		default:
			continue
		}
		if len(f.tracks) < 2 || f.defaultAudioOnly {
			lang := fileLanguage(f)
			menu = append(menu, PlaybackAudioTrack{ID: f.file.ID, Lang: lang, Label: audioLabel(lang), Default: own,
				Source: "file", StreamURL: stream, HLSURL: hls})
			continue
		}
		def := defaultTrack(f.tracks)
		for j, a := range f.tracks {
			label := a.Title
			if label == "" || label == a.Lang {
				label = audioLabel(a.Lang)
			}
			menu = append(menu, PlaybackAudioTrack{
				// the stream index alone repeats across files
				ID:   fmt.Sprintf("embedded:%s:%d", f.file.ID, a.Index),
				Lang: media.BCP47(a.Lang), Label: label, Default: own && j == def,
				Source: "embedded", StreamURL: stream, HLSURL: hls,
			})
		}
	}
	if len(menu) < 2 {
		return nil
	}
	return menu
}

// fileLanguage is what a file plays without a choice: the language it was
// tagged with, else its default track's.
func fileLanguage(f audioFile) string {
	if lang := media.BCP47(f.file.AudioLang); lang != "und" || len(f.tracks) == 0 {
		return lang
	}
	return media.BCP47(f.tracks[defaultTrack(f.tracks)].Lang)
}

// defaultTrack is the first track the file marks default, else its first; the
// HLS audio group marks the same one.
func defaultTrack(tracks []media.AudioStream) int {
	for i, a := range tracks {
		if a.Default {
			return i
		}
	}
	return 0
}
