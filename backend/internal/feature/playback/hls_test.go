package playback

import (
	"slices"
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
