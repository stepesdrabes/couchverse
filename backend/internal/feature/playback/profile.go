package playback

import (
	"slices"
	"strings"
)

// DeviceProfile is what a client can play, measured on the device at runtime
// (AVFoundation/VideoToolbox on Apple, MediaCodecList on Android,
// MediaSource.isTypeSupported in a browser). It is authoritative: the server
// never offers a stream the profile does not cover.
type DeviceProfile struct {
	Containers []string       `json:"containers" enum:"mp4,mov,mkv,webm,ts" doc:"Progressive containers the client direct-plays."`
	Video      []VideoSupport `json:"video" doc:"Video codecs the client decodes."`
	Audio      []AudioSupport `json:"audio" doc:"Audio codecs the client decodes or passes through to the receiver."`
	// Dolby Vision is listed per profile; dolbyVision8 stands for the whole family
	// (8.1 HDR10-, 8.2 SDR- and 8.4 HLG-compatible)
	HDR                 []string `json:"hdr,omitempty" enum:"hdr10,hdr10plus,hlg,dolbyVision5,dolbyVision7,dolbyVision8,dolbyVision10" doc:"HDR formats the client presents, Dolby Vision per profile; SDR is always assumed. HDR video outside the list plays its compatible base layer or is tone-mapped to SDR."`
	MaxWidth            int      `json:"maxWidth,omitempty" doc:"Widest video the client plays; 0 for no limit."`
	MaxHeight           int      `json:"maxHeight,omitempty" doc:"Tallest video the client plays; 0 for no limit."`
	MaxFrameRate        float64  `json:"maxFrameRate,omitempty" doc:"Highest frame rate the client plays; 0 for no limit."`
	MaxBitrate          int64    `json:"maxBitrate,omitempty" doc:"Bits per second the client can stream; 0 for no limit."`
	HLS                 []string `json:"hls" enum:"ts,fmp4" doc:"HLS segment formats the client plays; empty when it cannot play HLS."`
	SidecarSubtitles    []string `json:"sidecarSubtitles,omitempty" enum:"webvtt" doc:"Subtitle formats the client renders beside a direct-played file; without one, subtitles need HLS renditions."`
	AudioTrackSwitching bool     `json:"audioTrackSwitching,omitempty" doc:"The client switches between the audio tracks inside a direct-played file; without it, a file with several audio tracks plays through HLS."`
}

// VideoSupport is one decodable video codec.
type VideoSupport struct {
	Codec string `json:"codec" enum:"h264,hevc,av1,vp9"`
	// the vocabulary is the probe's normalized ffprobe profile name
	Profiles    []string `json:"profiles,omitempty" enum:"baseline,main,high,high10,high422,high444,main10,rext,professional,profile0,profile1,profile2,profile3" doc:"Decodable profiles; empty for every profile within maxBitDepth."`
	MaxLevel    float64  `json:"maxLevel,omitempty" doc:"Highest level as written (4.1, 5.1); 0 for any."`
	MaxBitDepth int      `json:"maxBitDepth,omitempty" doc:"Deepest decodable bit depth (8, 10, 12); 0 means 8."`
}

// AudioSupport is one decodable (or passthrough) audio codec.
type AudioSupport struct {
	Codec       string `json:"codec" enum:"aac,mp3,ac3,eac3,truehd,dts,flac,opus,vorbis,alac,pcm"`
	MaxChannels int    `json:"maxChannels,omitempty" doc:"Most channels the client outputs; 0 means 2."`
	Atmos       bool   `json:"atmos,omitempty" doc:"Dolby Atmos (E-AC-3 JOC, TrueHD) reaches the output as Atmos."`
}

// LegacyProfile is what the old GET form promised: the browser baseline
// (H.264, VP9 and AV1 in MP4/WebM with AAC, MP3, Opus, Vorbis or FLAC) plus the
// extra video codecs a client listed in ?caps=.
func LegacyProfile(caps []string) DeviceProfile {
	p := DeviceProfile{
		Containers: []string{"mp4", "webm"},
		Video: []VideoSupport{
			{Codec: "h264", Profiles: []string{"baseline", "main", "high"}},
			{Codec: "vp9", Profiles: []string{"profile0"}},
			{Codec: "av1", Profiles: []string{"main"}},
		},
		Audio: []AudioSupport{
			{Codec: "aac", MaxChannels: 6}, {Codec: "mp3"}, {Codec: "opus", MaxChannels: 6},
			{Codec: "vorbis"}, {Codec: "flac", MaxChannels: 6},
		},
		HLS:              []string{"ts", "fmp4"},
		SidecarSubtitles: []string{"webvtt"},
	}
	for _, c := range caps {
		if c == "hevc" && !slices.ContainsFunc(p.Video, func(v VideoSupport) bool { return v.Codec == "hevc" }) {
			p.Video = append(p.Video, VideoSupport{Codec: "hevc", Profiles: []string{"main", "main10"}, MaxBitDepth: 10})
		}
	}
	return p
}

func (p DeviceProfile) video(codec string) *VideoSupport {
	for i := range p.Video {
		if p.Video[i].Codec == codec {
			return &p.Video[i]
		}
	}
	return nil
}

func (p DeviceProfile) audio(codec string) *AudioSupport {
	for i := range p.Audio {
		if p.Audio[i].Codec == codec {
			return &p.Audio[i]
		}
	}
	return nil
}

func (p DeviceProfile) hdr(mode string) bool { return slices.Contains(p.HDR, mode) }

// playsHLS reports whether the client plays fMP4 HLS, which every prepared
// stream and instant-play session is.
func (p DeviceProfile) playsHLS() bool { return slices.Contains(p.HLS, "fmp4") }

// playsAudio reports whether a track reaches the client as is.
func (p DeviceProfile) playsAudio(codec string, channels int) bool {
	a := p.audio(audioCodecName(codec))
	if a == nil {
		return false
	}
	return channels <= max(a.MaxChannels, 2)
}

// audioCodecName maps ffprobe codec names onto the profile vocabulary.
func audioCodecName(codec string) string {
	switch {
	case strings.HasPrefix(codec, "pcm_"):
		return "pcm"
	case codec == "dca":
		return "dts"
	}
	return codec
}

// supportsVideo reports whether the client decodes the codec at this
// profile, level and bit depth.
func (s *VideoSupport) supports(profile string, level float64, bitDepth int) bool {
	if s == nil {
		return false
	}
	if bitDepth > max(s.MaxBitDepth, 8) {
		return false
	}
	if len(s.Profiles) > 0 && profile != "" && !slices.Contains(s.Profiles, profile) {
		return false
	}
	return s.MaxLevel == 0 || level == 0 || level <= s.MaxLevel+0.001
}
