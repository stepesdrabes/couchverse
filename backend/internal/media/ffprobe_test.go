package media

import (
	"slices"
	"testing"
)

// probeJSON is trimmed ffprobe output of a Dolby Vision 8.1 MKV with E-AC-3
// Atmos, a stereo AAC track and a forced subtitle.
const probeJSON = `{
  "format": {"format_name": "matroska,webm", "duration": "20.000", "bit_rate": "12000000"},
  "streams": [
    {"index": 0, "codec_type": "video", "codec_name": "hevc", "codec_tag_string": "[0][0][0][0]",
     "profile": "Main 10", "level": 153, "pix_fmt": "yuv420p10le", "avg_frame_rate": "24000/1001",
     "r_frame_rate": "24000/1001", "width": 3840, "height": 2160, "color_transfer": "smpte2084",
     "side_data_list": [{"side_data_type": "Mastering display metadata"},
       {"side_data_type": "DOVI configuration record", "dv_profile": 8, "dv_bl_signal_compatibility_id": 1}]},
    {"index": 1, "codec_type": "audio", "codec_name": "eac3", "profile": "Dolby Digital Plus + Dolby Atmos",
     "channels": 6, "channel_layout": "5.1(side)", "sample_rate": "48000", "disposition": {"default": 1},
     "tags": {"language": "eng", "title": "English Atmos"}},
    {"index": 2, "codec_type": "audio", "codec_name": "aac", "profile": "LC", "channels": 2,
     "channel_layout": "stereo", "sample_rate": "48000", "tags": {"language": "ces"}},
    {"index": 3, "codec_type": "subtitle", "codec_name": "subrip", "disposition": {"forced": 1},
     "tags": {"language": "ces"}}
  ]
}`

func TestParseProbe(t *testing.T) {
	res, err := ParseProbe([]byte(probeJSON), "/movies/Prism (2020).mkv")
	if err != nil {
		t.Fatal(err)
	}
	if res.Container != "mkv" || res.VideoCodec != "hevc" || res.VideoRange != "dv" {
		t.Errorf("container %s codec %s range %s", res.Container, res.VideoCodec, res.VideoRange)
	}
	want := VideoStream{Profile: "main10", Level: 5.1, BitDepth: 10, FrameRate: 24000.0 / 1001,
		HDR: HDRDolbyVision, DoviProfile: 8, DoviCompatibility: 1}
	if res.Video != want {
		t.Errorf("video %+v\nwant  %+v", res.Video, want)
	}
	if len(res.AudioStreams) != 2 || !res.AudioStreams[0].Atmos() || res.AudioStreams[1].Atmos() ||
		res.AudioStreams[0].ChannelLayout != "5.1(side)" || !res.AudioStreams[0].Default {
		t.Errorf("audio %+v", res.AudioStreams)
	}
	if len(res.SubtitleStreams) != 1 || !res.SubtitleStreams[0].Forced {
		t.Errorf("subtitles %+v", res.SubtitleStreams)
	}
}

func TestNormalize(t *testing.T) {
	for _, c := range []struct {
		codec, profile string
		level          int
		wantProfile    string
		wantLevel      float64
	}{
		{"h264", "Constrained Baseline", 31, "baseline", 3.1},
		{"h264", "High 10", 51, "high10", 5.1},
		{"hevc", "Main", 120, "main", 4},
		{"hevc", "Rext", 186, "rext", 6.2},
		{"av1", "Main", 13, "main", 5.1},
		{"vp9", "Profile 2", -99, "profile2", 0},
	} {
		if p, l := normalizeProfile(c.profile), normalizeLevel(c.codec, c.level); p != c.wantProfile || l != c.wantLevel {
			t.Errorf("%s %s %d: got %s %.1f, want %s %.1f", c.codec, c.profile, c.level, p, l, c.wantProfile, c.wantLevel)
		}
	}
}

func TestAutoPrepare(t *testing.T) {
	settings := TranscodeSettings{Ladder: []string{"1080p", "720p"}}
	h264 := func(container string, height int) *ProbeResult {
		return &ProbeResult{Container: container, HasVideo: true, VideoCodec: "h264", Height: height,
			AudioCodec: "aac", AudioStreams: []AudioStream{{Codec: "aac"}}, Video: VideoStream{BitDepth: 8, HDR: HDRNone}}
	}
	hdr := h264("mkv", 2160)
	hdr.VideoCodec, hdr.Video = "hevc", VideoStream{BitDepth: 10, HDR: HDR10}
	vp9 := h264("webm", 1080)
	vp9.VideoCodec = "vp9"
	scope := h264("mkv", 800)
	scope.Width, scope.VideoCodec, scope.Video = 1920, "hevc", VideoStream{BitDepth: 10, HDR: HDR10}
	off := TranscodeSettings{Ladder: []string{"720p"}, AutoPrepare: new(bool)}

	for _, c := range []struct {
		name     string
		probe    *ProbeResult
		settings TranscodeSettings
		want     []string
	}{
		{"mp4 every client plays", h264("mp4", 1080), settings, nil},
		{"mkv gets its copy and the rungs below it", h264("mkv", 1080), settings, []string{VariantSource, "720p"}},
		{"hdr gets its copy and the whole ladder", hdr, settings, []string{VariantSource, "1080p", "720p"}},
		{"vp9 cannot be copied", vp9, settings, []string{"1080p", "720p"}},
		{"a 1920x800 film is 1080p", scope, settings, []string{VariantSource, "1080p", "720p"}},
		{"auto-prepare off still copies", hdr, off, []string{VariantSource}},
	} {
		if got := AutoPrepare(c.probe, c.settings); !slices.Equal(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}
