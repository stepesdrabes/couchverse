package playback

import (
	"slices"
	"strings"
	"testing"

	"couchverse/internal/media"
)

// argValue returns the argument following flag, or "" when absent.
func argValue(args []string, flag string) string {
	i := slices.Index(args, flag)
	if i < 0 || i+1 >= len(args) {
		return ""
	}
	return args[i+1]
}

func TestMultiAudioMapsSourceIndexesAndOneDefault(t *testing.T) {
	args := BuildArgs(BuildSpec{
		Mode: "copy", OutDir: "/output", MultiAudio: true,
		AudioStreams: []media.AudioStream{
			{Index: 2, Lang: "eng", Default: true},
			{Index: 5, Lang: "ces", Default: true},
		},
	})
	maps := []string{}
	for i, arg := range args {
		if arg == "-map" && i+1 < len(args) {
			maps = append(maps, args[i+1])
		}
	}
	if !slices.Equal(maps, []string{"0:v:0", "0:2", "0:5"}) {
		t.Errorf("global input stream indexes must be mapped: %v", maps)
	}
	streamMap := argValue(args, "-var_stream_map")
	if !strings.Contains(streamMap, "a:0,agroup:aud,language:eng") || !strings.Contains(streamMap, "a:1,agroup:aud,language:ces") {
		t.Errorf("HLS audio indexes must match menu output indexes: %s", streamMap)
	}
	if strings.Count(streamMap, "default:yes") != 1 {
		t.Errorf("HLS must contain exactly one default, even if the input flags several: %s", streamMap)
	}
	if got := argValue(args, "-c:v"); got != "copy" {
		t.Errorf("multiaudio preparation must copy video instead of encoding it: %s", got)
	}
}

func TestBuildArgsBitrateCap(t *testing.T) {
	rendition := media.Renditions["1080p"].CappedAt(3_500_000)

	t.Run("libx264 maxrate and bufsize follow the capped bitrate", func(t *testing.T) {
		args := BuildArgs(BuildSpec{
			Mode: "transcode", Rendition: rendition, Encoder: "libx264", Preset: "veryfast",
		})
		if got := argValue(args, "-maxrate"); got != "3500000" {
			t.Errorf("-maxrate = %s, want 3500000", got)
		}
		if got := argValue(args, "-bufsize"); got != "7000000" {
			t.Errorf("-bufsize = %s, want 7000000", got)
		}
	})

	t.Run("hardware encoders target the capped bitrate", func(t *testing.T) {
		args := BuildArgs(BuildSpec{
			Mode: "transcode", Rendition: rendition, Encoder: "h264_videotoolbox",
		})
		if got := argValue(args, "-b:v"); got != "3500000" {
			t.Errorf("-b:v = %s, want 3500000", got)
		}
		if got := argValue(args, "-maxrate"); got != "3500000" {
			t.Errorf("-maxrate = %s, want 3500000", got)
		}
	})

	t.Run("copy mode has no rate control", func(t *testing.T) {
		args := BuildArgs(BuildSpec{Mode: "copy", Rendition: rendition})
		if slices.Contains(args, "-maxrate") || slices.Contains(args, "-b:v") {
			t.Errorf("copy mode must not set rate-control flags: %v", args)
		}
	})
}

func TestBuildArgsFitsTheRungBox(t *testing.T) {
	rendition := media.Renditions["720p"]

	t.Run("a scope film keeps the box width", func(t *testing.T) {
		args := BuildArgs(BuildSpec{Mode: "transcode", Rendition: rendition, Encoder: "libx264",
			SourceWidth: 1920, SourceHeight: 800})
		if got := argValue(args, "-vf"); got != "scale=1280:532" {
			t.Errorf("-vf = %s, want scale=1280:532", got)
		}
	})

	t.Run("an unknown source scales to the box height", func(t *testing.T) {
		args := BuildArgs(BuildSpec{Mode: "transcode", Rendition: rendition, Encoder: "libx264"})
		if got := argValue(args, "-vf"); got != "scale=-2:720" {
			t.Errorf("-vf = %s, want scale=-2:720", got)
		}
	})
}
