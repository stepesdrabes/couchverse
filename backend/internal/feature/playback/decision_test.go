package playback

import (
	"slices"
	"testing"

	"couchverse/internal/media"
)

// Device profiles as the clients measure them.
var (
	chrome = DeviceProfile{
		Containers: []string{"mp4", "webm"},
		Video: []VideoSupport{
			{Codec: "h264", Profiles: []string{"baseline", "main", "high"}, MaxLevel: 5.2},
			{Codec: "vp9", Profiles: []string{"profile0"}},
			{Codec: "av1", Profiles: []string{"main"}},
		},
		Audio:            []AudioSupport{{Codec: "aac", MaxChannels: 6}, {Codec: "mp3"}, {Codec: "opus", MaxChannels: 6}, {Codec: "flac", MaxChannels: 6}},
		HLS:              []string{"ts", "fmp4"},
		SidecarSubtitles: []string{"webvtt"},
	}
	appleTV = DeviceProfile{
		Containers: []string{"mp4", "mov"},
		Video: []VideoSupport{
			{Codec: "h264", MaxLevel: 5.2},
			{Codec: "hevc", Profiles: []string{"main", "main10"}, MaxLevel: 5.1, MaxBitDepth: 10},
		},
		Audio:    []AudioSupport{{Codec: "aac", MaxChannels: 6}, {Codec: "ac3", MaxChannels: 6}, {Codec: "eac3", MaxChannels: 8, Atmos: true}, {Codec: "flac", MaxChannels: 8}},
		HDR:      []string{"hdr10", "hlg", "dolbyVision5", "dolbyVision8"},
		MaxWidth: 3840, MaxHeight: 2160, MaxFrameRate: 60,
		HLS:                 []string{"ts", "fmp4"},
		AudioTrackSwitching: true,
	}
	sdrPhone = DeviceProfile{
		Containers: []string{"mp4", "mov"},
		Video:      []VideoSupport{{Codec: "h264", MaxLevel: 5.1}, {Codec: "hevc", Profiles: []string{"main"}, MaxLevel: 5.1}},
		Audio:      []AudioSupport{{Codec: "aac", MaxChannels: 2}},
		MaxWidth:   1920, MaxHeight: 1080,
		MaxBitrate:          8_000_000,
		HLS:                 []string{"fmp4"},
		AudioTrackSwitching: true,
	}
	androidTV = DeviceProfile{
		Containers: []string{"mp4", "mkv", "webm"},
		Video: []VideoSupport{
			{Codec: "h264"}, {Codec: "hevc", MaxBitDepth: 10}, {Codec: "vp9", MaxBitDepth: 10}, {Codec: "av1", MaxBitDepth: 10},
		},
		Audio:               []AudioSupport{{Codec: "aac", MaxChannels: 6}, {Codec: "ac3", MaxChannels: 6}, {Codec: "eac3", MaxChannels: 6}, {Codec: "opus", MaxChannels: 6}},
		HDR:                 []string{"hdr10", "hdr10plus", "hlg", "dolbyVision7", "dolbyVision8"},
		HLS:                 []string{"fmp4"},
		SidecarSubtitles:    []string{"webvtt"},
		AudioTrackSwitching: true,
	}
)

func audio(index int, codec string, channels int, extra ...string) media.AudioStream {
	a := media.AudioStream{Index: index, Codec: codec, Channels: channels, Default: index == 1}
	if len(extra) > 0 {
		a.Profile = extra[0]
	}
	return a
}

func video(codec, profile string, depth int, hdr string) media.VideoStream {
	return media.VideoStream{Profile: profile, Level: 4.1, BitDepth: depth, FrameRate: 23.976, HDR: hdr}
}

// Sample media, mirroring scripts/gen-sample-media.sh.
var (
	h264MP4 = Source{Container: "mp4", VideoCodec: "h264", Video: video("h264", "high", 8, media.HDRNone),
		Width: 1920, Height: 1080, Bitrate: 5_000_000, Audio: []media.AudioStream{audio(1, "aac", 2)}, Available: true}
	h264MP4Subs = with(h264MP4, func(s *Source) { s.Subtitles = 2 })
	h264AC3MKV  = with(h264MP4, func(s *Source) {
		s.Container, s.Audio = "mkv", []media.AudioStream{audio(1, "ac3", 6)}
	})
	hevcHDR10MKV = Source{Container: "mkv", VideoCodec: "hevc", Video: video("hevc", "main10", 10, media.HDR10),
		Width: 3840, Height: 2160, Bitrate: 20_000_000, Subtitles: 2, Available: true,
		Audio: []media.AudioStream{audio(1, "eac3", 6, "Dolby Digital Plus + Dolby Atmos"), audio(2, "aac", 2)}}
	hevcSDRMP4 = Source{Container: "mp4", VideoCodec: "hevc", Video: video("hevc", "main", 8, media.HDRNone),
		Width: 1920, Height: 1080, Bitrate: 4_000_000, Audio: []media.AudioStream{audio(1, "aac", 2)}, Available: true}
	vp9WebM = Source{Container: "webm", VideoCodec: "vp9", Video: video("vp9", "profile0", 8, media.HDRNone),
		Width: 1920, Height: 1080, Bitrate: 3_000_000, Audio: []media.AudioStream{audio(1, "opus", 2)}, Available: true}
	dv81MP4 = Source{Container: "mp4", VideoCodec: "hevc", Bitrate: 15_000_000, Width: 3840, Height: 2160, Available: true,
		Video: media.VideoStream{Profile: "main10", Level: 5.1, BitDepth: 10, FrameRate: 24, HDR: media.HDRDolbyVision, DoviProfile: 8, DoviCompatibility: 1},
		Audio: []media.AudioStream{audio(1, "eac3", 6)}}
	dv5MP4 = with(dv81MP4, func(s *Source) { s.Video.DoviProfile, s.Video.DoviCompatibility = 5, 0 })
	dv7MKV = with(dv81MP4, func(s *Source) {
		s.Container, s.Video.DoviProfile, s.Video.DoviCompatibility = "mkv", 7, 6
		s.Audio = []media.AudioStream{audio(1, "truehd", 8, "Dolby TrueHD + Dolby Atmos")}
	})
	hlgMKV = with(hevcHDR10MKV, func(s *Source) { s.Video.HDR, s.Subtitles = media.HLG, 0 })
)

func with(s Source, edit func(*Source)) Source {
	s.Audio = slices.Clone(s.Audio)
	edit(&s)
	return s
}

var (
	nothing    = Prepared{}
	packaged   = Prepared{Original: PackageReady, Audio: PackageReady}
	laddered   = Prepared{Audio: PackageReady, Ladder: PackageReady}
	everything = Prepared{Original: PackageReady, Audio: PackageReady, Ladder: PackageReady}
	preparing  = Prepared{Original: PackagePending, Audio: PackagePending, Ladder: PackagePending}
	legacy     = Prepared{Legacy: PackageReady}
	jitOn      = Server{JIT: true, DolbyVisionCopy: true}
	jitOff     = Server{DolbyVisionCopy: true}
)

func TestDecide(t *testing.T) {
	cases := []struct {
		name    string
		profile DeviceProfile
		source  Source
		prep    Prepared
		srv     Server
		want    Decision
	}{
		// the baseline file direct-plays everywhere
		{"chrome h264 mp4", chrome, h264MP4, nothing, jitOff, Decision{Tier: TierDirect, Mode: "direct"}},
		{"apple tv h264 mp4", appleTV, h264MP4, nothing, jitOff, Decision{Tier: TierDirect, Mode: "direct"}},
		// AVPlayer has no sidecar subtitles: remux for the renditions once
		// prepared, direct without them until then
		{"apple tv subtitles packaged", appleTV, h264MP4Subs, packaged, jitOff,
			Decision{Tier: TierRemux, Mode: "hls", Master: Master{Video: "original", Surround: []string{"eac3", "ac3"}}}},
		{"apple tv subtitles not packaged", appleTV, h264MP4Subs, preparing, jitOff, Decision{Tier: TierDirect, Mode: "direct"}},
		{"chrome subtitles stay sidecar", chrome, h264MP4Subs, packaged, jitOff, Decision{Tier: TierDirect, Mode: "direct"}},
		// MKV with AC-3: browsers remux and get AAC, Android TV plays it as is
		{"chrome mkv ac3", chrome, h264AC3MKV, packaged, jitOff,
			Decision{Tier: TierRemux, Mode: "hls", Master: Master{Video: "original", Surround: []string{}}}},
		{"android tv mkv ac3", androidTV, h264AC3MKV, nothing, jitOff, Decision{Tier: TierDirect, Mode: "direct"}},
		// HEVC HDR10: copied for HDR clients, tone-mapped ladder for the rest
		{"apple tv hdr10 mkv", appleTV, hevcHDR10MKV, everything, jitOff,
			Decision{Tier: TierRemux, Mode: "hls", Master: Master{Video: "original", Surround: []string{"eac3", "ac3"}}, Ladder: true}},
		{"chrome hdr10 mkv", chrome, hevcHDR10MKV, everything, jitOff,
			Decision{Tier: TierTranscode, Mode: "hls", Master: Master{Video: "ladder", Surround: []string{}}, Ladder: true}},
		{"sdr phone hdr10 mkv", sdrPhone, hevcHDR10MKV, everything, jitOff,
			Decision{Tier: TierTranscode, Mode: "hls", Master: Master{Video: "ladder", Surround: []string{}}, Ladder: true}},
		{"android tv hdr10 mkv", androidTV, hevcHDR10MKV, nothing, jitOff, Decision{Tier: TierDirect, Mode: "direct"}},
		{"chrome hdr10 preparing", chrome, hevcHDR10MKV, preparing, jitOn, Decision{Mode: "preparing"}},
		{"chrome hdr10 jit", chrome, hevcHDR10MKV, nothing, jitOn,
			Decision{Tier: TierTranscode, Mode: "jit", JIT: &JITPlan{Video: "transcode", AudioStream: 1, Audio: "aac"}}},
		{"apple tv hdr10 jit copies", appleTV, hevcHDR10MKV, nothing, jitOn,
			Decision{Tier: TierRemux, Mode: "jit", JIT: &JITPlan{Video: "copy", AudioStream: 1, Audio: "copy"}}},
		{"chrome hdr10 nothing", chrome, hevcHDR10MKV, nothing, jitOff, Decision{Mode: "unsupported"}},
		// HLG needs an HLG display
		{"apple tv hlg", appleTV, hlgMKV, packaged, jitOff,
			Decision{Tier: TierRemux, Mode: "hls", Master: Master{Video: "original", Surround: []string{"eac3", "ac3"}}}},
		{"android tv hlg", androidTV, hlgMKV, nothing, jitOff, Decision{Tier: TierDirect, Mode: "direct"}},
		// a bitrate ceiling rules out the source and its copy
		{"sdr phone over bitrate", sdrPhone, with(hevcSDRMP4, func(s *Source) { s.Bitrate = 12_000_000 }), everything, jitOff,
			Decision{Tier: TierTranscode, Mode: "hls", Master: Master{Video: "ladder", Surround: []string{}}, Ladder: true}},
		{"sdr phone hevc mp4", sdrPhone, hevcSDRMP4, nothing, jitOff, Decision{Tier: TierDirect, Mode: "direct"}},
		// VP9/WebM/Opus: never "direct" for Apple (the old caps bug)
		{"apple tv vp9 webm", appleTV, vp9WebM, laddered, jitOff,
			Decision{Tier: TierTranscode, Mode: "hls", Master: Master{Video: "ladder", Surround: []string{"eac3", "ac3"}}, Ladder: true}},
		{"apple tv vp9 webm nothing", appleTV, vp9WebM, nothing, jitOff, Decision{Mode: "unsupported"}},
		{"chrome vp9 webm", chrome, vp9WebM, nothing, jitOff, Decision{Tier: TierDirect, Mode: "direct"}},
		// Dolby Vision 8.1: as DV where decoded, else its HDR10 base layer, else tone-mapped
		{"apple tv dv8.1", appleTV, dv81MP4, nothing, jitOff, Decision{Tier: TierDirect, Mode: "direct"}},
		{"android tv dv8.1", androidTV, dv81MP4, nothing, jitOff, Decision{Tier: TierDirect, Mode: "direct"}},
		{"chrome dv8.1", chrome, dv81MP4, everything, jitOff,
			Decision{Tier: TierTranscode, Mode: "hls", Master: Master{Video: "ladder", Surround: []string{}}, Ladder: true}},
		// profile 5 has no base layer, so a client without DV 5 transcodes
		{"apple tv dv5", appleTV, dv5MP4, nothing, jitOff, Decision{Tier: TierDirect, Mode: "direct"}},
		{"android tv dv5", androidTV, dv5MP4, everything, jitOff,
			Decision{Tier: TierTranscode, Mode: "hls", Master: Master{Video: "ladder", Surround: []string{"eac3", "ac3"}}, Ladder: true}},
		{"apple tv dv5 no dv copy", appleTV, with(dv5MP4, func(s *Source) { s.Container = "mkv" }), everything,
			Server{DolbyVisionCopy: false},
			Decision{Tier: TierTranscode, Mode: "hls", Master: Master{Video: "ladder", Surround: []string{"eac3", "ac3"}}, Ladder: true}},
		// profile 7: Apple plays the HDR10 base layer of the prepared copy,
		// Android TV decodes the dual layer from the MKV itself
		{"apple tv dv7", appleTV, dv7MKV, packaged, jitOff,
			Decision{Tier: TierRemux, Mode: "hls", Master: Master{Video: "original", Surround: []string{"eac3", "ac3"}}}},
		{"android tv dv7 truehd", androidTV, dv7MKV, packaged, jitOff,
			Decision{Tier: TierRemux, Mode: "hls", Master: Master{Video: "original", Surround: []string{"eac3", "ac3"}}}},
		// legacy MPEG-TS variants keep playing until prepared again
		{"chrome legacy", chrome, hevcHDR10MKV, legacy, jitOff, Decision{Tier: TierTranscode, Mode: "hls", Master: Master{Video: "legacy"}}},
		{"sdr phone no ts", sdrPhone, hevcHDR10MKV, legacy, jitOff, Decision{Mode: "unsupported"}},
		// a deleted source leaves only what was prepared
		{"chrome deleted source", chrome, with(h264MP4, func(s *Source) { s.Available = false }), laddered, jitOn,
			Decision{Tier: TierTranscode, Mode: "hls", Master: Master{Video: "ladder", Surround: []string{}}, Ladder: true}},
		// the old GET form keeps its meaning
		{"legacy caps hevc", LegacyProfile([]string{"hevc"}), hevcSDRMP4, nothing, jitOff, Decision{Tier: TierDirect, Mode: "direct"}},
		{"legacy caps no hevc", LegacyProfile(nil), hevcSDRMP4, nothing, jitOn,
			Decision{Tier: TierTranscode, Mode: "jit", JIT: &JITPlan{Video: "transcode", AudioStream: 1, Audio: "copy"}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Decide(c.profile, c.source, c.prep, c.srv)
			if !equalDecision(got, c.want) {
				t.Errorf("got  %+v %+v\nwant %+v %+v", got, got.JIT, c.want, c.want.JIT)
			}
		})
	}
}

func equalDecision(a, b Decision) bool {
	if a.Tier != b.Tier || a.Mode != b.Mode || a.Ladder != b.Ladder || a.Master.Video != b.Master.Video {
		return false
	}
	if len(a.Master.Surround) != len(b.Master.Surround) || !slices.Equal(a.Master.Surround, b.Master.Surround) {
		return false
	}
	if (a.JIT == nil) != (b.JIT == nil) {
		return false
	}
	return a.JIT == nil || *a.JIT == *b.JIT
}
