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

func TestRungBitrateCap(t *testing.T) {
	mf := &media.MediaFile{VideoCodec: "h264", Height: 1080, Video: media.VideoStream{FrameRate: 25}}
	rendition := media.Renditions["1080p"].CappedAt(3_500_000)

	t.Run("libx264 maxrate and bufsize follow the capped bitrate", func(t *testing.T) {
		args := rungVideoArgs(mf, rendition, "libx264", "veryfast", Features{}, 0)
		if got := argValue(args, "-maxrate"); got != "3500000" {
			t.Errorf("-maxrate = %s, want 3500000", got)
		}
		if got := argValue(args, "-bufsize"); got != "7000000" {
			t.Errorf("-bufsize = %s, want 7000000", got)
		}
	})

	t.Run("hardware encoders target the capped bitrate", func(t *testing.T) {
		args := rungVideoArgs(mf, rendition, "h264_videotoolbox", "", Features{}, 0)
		if got := argValue(args, "-b:v"); got != "3500000" {
			t.Errorf("-b:v = %s, want 3500000", got)
		}
	})

	t.Run("an IDR every two seconds of source time, from the start position", func(t *testing.T) {
		args := rungVideoArgs(mf, rendition, "libx264", "veryfast", Features{}, 36)
		if got := argValue(args, "-force_key_frames"); got != "expr:gte(t,36+n_forced*2)" {
			t.Errorf("-force_key_frames = %s", got)
		}
		if got := argValue(args, "-g"); got != "50" {
			t.Errorf("-g = %s, want 50", got)
		}
	})

	t.Run("copying has no rate control", func(t *testing.T) {
		args := sourceVideoArgs(mf, Features{})
		if slices.Contains(args, "-maxrate") || slices.Contains(args, "-b:v") {
			t.Errorf("copy must not set rate-control flags: %v", args)
		}
	})
}

func TestRungVideoFilter(t *testing.T) {
	hdr := &media.MediaFile{VideoCodec: "hevc", Video: media.VideoStream{BitDepth: 10, HDR: media.HDR10}}
	sdr10 := &media.MediaFile{VideoCodec: "hevc", Video: media.VideoStream{BitDepth: 10, HDR: media.HDRNone}}
	cases := []struct {
		name string
		mf   *media.MediaFile
		f    Features
		want []string
	}{
		{"hdr is tone-mapped", hdr, Features{ToneMap: true}, []string{"scale=-2:720", "tonemap=tonemap=hable", "format=yuv420p"}},
		{"hdr without zscale is relabelled sdr and ends 8-bit", hdr, Features{}, []string{"color_trc=bt709", "format=yuv420p"}},
		// the old pipeline handed 10-bit to libx264, which made High 10 nothing plays
		{"10-bit sdr ends 8-bit", sdr10, Features{ToneMap: true}, []string{"scale=-2:720,format=yuv420p"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := videoFilter(c.mf, 720, c.f)
			for _, w := range c.want {
				if !strings.Contains(got, w) {
					t.Errorf("filter %q lacks %q", got, w)
				}
			}
		})
	}
}

func TestSourceVideoArgs(t *testing.T) {
	dovi := func(profile, compat int) *media.MediaFile {
		return &media.MediaFile{VideoCodec: "hevc", Video: media.VideoStream{BitDepth: 10, HDR: media.HDRDolbyVision,
			DoviProfile: profile, DoviCompatibility: compat}}
	}
	modern, old := Features{DolbyVision: true}, Features{}
	cases := []struct {
		name string
		mf   *media.MediaFile
		f    Features
		want string
	}{
		{"hevc is tagged hvc1 for AVPlayer", &media.MediaFile{VideoCodec: "hevc"}, modern, "-tag:v hvc1"},
		{"h264 keeps its tag", &media.MediaFile{VideoCodec: "h264"}, modern, "-c:v copy"},
		{"profile 8.1 stays hvc1 with its DV config", dovi(8, 1), modern, "-tag:v hvc1 -strict unofficial"},
		{"profile 8.1 on ffmpeg 5 is its HDR10 base layer", dovi(8, 1), old, "-tag:v hvc1"},
		{"profile 5 is dvh1", dovi(5, 0), modern, "-tag:v dvh1 -strict unofficial"},
		{"profile 7 drops its enhancement layer and RPUs", dovi(7, 6), modern, "-tag:v hvc1 -bsf:v filter_units=remove_types=62|63"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := strings.Join(sourceVideoArgs(c.mf, c.f), " ")
			if !strings.HasSuffix(got, c.want) || strings.Contains(c.name, "base layer") && strings.Contains(got, "strict") {
				t.Errorf("got %q, want it to end with %q", got, c.want)
			}
		})
	}
	if canCopySource(dovi(5, 0), old) {
		t.Error("profile 5 cannot be copied without dvh1")
	}
	if !canCopySource(dovi(7, 6), old) {
		t.Error("profile 7 copies as its base layer")
	}
}

func TestAudioRenditions(t *testing.T) {
	tracks := []media.AudioStream{
		{Index: 1, Codec: "eac3", Channels: 6, Profile: "Dolby Digital Plus + Dolby Atmos"},
		{Index: 2, Codec: "aac", Channels: 2},
		{Index: 3, Codec: "truehd", Channels: 8},
		{Index: 4, Codec: "dts", Channels: 6},
	}
	var got []string
	for _, r := range audioRenditions(tracks) {
		mode := "encode"
		if r.copy {
			mode = "copy"
		}
		got = append(got, r.dir()+":"+mode)
	}
	want := []string{
		"audio-1-aac:encode", "audio-1-eac3:copy", // E-AC-3 Atmos passes through
		"audio-2-aac:copy",
		"audio-3-aac:encode", "audio-3-eac3:encode", // TrueHD becomes E-AC-3 5.1
		"audio-4-aac:encode", "audio-4-eac3:encode",
	}
	if !slices.Equal(got, want) {
		t.Errorf("got  %v\nwant %v", got, want)
	}
}

func TestBCP47(t *testing.T) {
	for in, want := range map[string]string{"eng": "en", "cze": "cs", "ces": "cs", "en": "en", "": "und", "xx-bogus": "und", "und": "und"} {
		if got := media.BCP47(in); got != want {
			t.Errorf("media.BCP47(%q) = %q, want %q", in, got, want)
		}
	}
}
