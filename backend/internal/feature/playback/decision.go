package playback

import (
	"slices"
	"strconv"

	"couchverse/internal/media"
)

// Delivery tiers, cheapest first.
const (
	TierDirect    = "direct"    // the source file as is
	TierRemux     = "remux"     // the source video copied into fMP4 HLS
	TierTranscode = "transcode" // the H.264 SDR ladder (or a JIT transcode)
)

// Source is what the decision knows about a media file.
type Source struct {
	Container  string
	VideoCodec string
	Video      media.VideoStream
	Width      int
	Height     int
	Bitrate    int64
	// Audio lists the embedded audio tracks.
	Audio []media.AudioStream
	// Subtitles counts the sidecar WebVTT tracks.
	Subtitles int
	// Available is false once the source was deleted after transcoding.
	Available bool
}

// PackageState is how far one HLS package of a file has come.
type PackageState int

const (
	PackageMissing PackageState = iota
	PackagePending
	PackageReady
)

// Prepared summarizes a file's prepared HLS packages.
type Prepared struct {
	// Original is the copied source video, Audio the audio renditions; both
	// fMP4 (HLS v2).
	Original, Audio PackageState
	// Ladder is the best state among the transcoded fMP4 renditions.
	Ladder PackageState
	// Legacy is the best state among MPEG-TS variants prepared before HLS v2.
	Legacy PackageState
	// LegacyMultiAudio is a ready MPEG-TS remux carrying every audio language.
	LegacyMultiAudio bool
}

// Server is what the decision needs to know about this server.
type Server struct {
	// JIT allows instant-play sessions.
	JIT bool
	// DolbyVisionCopy is true when ffmpeg can signal Dolby Vision in fMP4
	// (dvh1/dvvC); without it a copied profile 8 stream plays as its base layer
	// and profile 5 cannot be copied at all.
	DolbyVisionCopy bool
}

// Master selects one of a file's multivariant playlists.
type Master struct {
	// Video is "original" (the copied source, outside the ABR ladder),
	// "ladder" (the transcoded renditions) or "legacy" (pre-v2 MPEG-TS).
	Video string
	// Surround lists the multichannel audio codecs the client takes (ac3,
	// eac3); AAC stereo is always offered.
	Surround []string
}

// JITPlan is the instant-play session to open for a file nothing is prepared for.
type JITPlan struct {
	Video       string `json:"video" enum:"copy,transcode" doc:"transcode makes H.264 SDR; copy (keeping the source video) is reserved for a later server."`
	AudioStream int    `json:"audioStream" doc:"Source stream index of the audio track to play; -1 for none."`
	Audio       string `json:"audio" enum:"copy,aac,eac3" doc:"copy keeps the track's codec; aac makes stereo, eac3 5.1."`
}

// Decision is how a client plays a file.
type Decision struct {
	Tier string
	// Mode is direct, hls, jit, preparing or unsupported (PlaybackInfo.Mode).
	Mode   string
	Master Master
	JIT    *JITPlan
	// Ladder is set when the transcoded ladder is ready and suits the client,
	// for the player's quality menu next to whatever plays.
	Ladder bool
}

// Decide picks the cheapest tier the profile can play from what is prepared.
// The profile is authoritative: no tier is offered that it does not cover.
func Decide(p DeviceProfile, s Source, prep Prepared, srv Server) Decision {
	video := fitVideo(p, s, srv)
	surround := surroundCodecs(p)
	original := video.original && p.playsHLS() && withinBitrate(p, s.Bitrate)
	// a file without audio has no audio package to wait for
	audioReady := len(s.Audio) == 0 || prep.Audio == PackageReady
	// transcodes are H.264 High, 8-bit SDR, at most 1080p
	transcoded := p.video("h264").supports("high", 4.1, 8)
	ladder := transcoded && p.playsHLS() && prep.Ladder == PackageReady && audioReady
	directVideo := s.Available && video.direct && withinBitrate(p, s.Bitrate) &&
		slices.Contains(p.Containers, containerName(s.Container))

	switch {
	case directVideo && audioDirect(p, s) && !needsRenditions(p, s):
		return Decision{Tier: TierDirect, Mode: "direct", Ladder: ladder}
	case original && prep.Original == PackageReady && audioReady:
		return Decision{Tier: TierRemux, Mode: "hls", Master: Master{Video: "original", Surround: surround}, Ladder: ladder}
	case ladder:
		return Decision{Tier: TierTranscode, Mode: "hls", Master: Master{Video: "ladder", Surround: surround}, Ladder: true}
	case transcoded && slices.Contains(p.HLS, "ts") && prep.Legacy == PackageReady:
		return Decision{Tier: TierTranscode, Mode: "hls", Master: Master{Video: "legacy"}}
	}

	// nothing prepared fits yet; a source that direct-plays except for its
	// subtitles or extra audio languages plays now rather than waiting for them
	pending := original && (prep.Original == PackagePending || prep.Audio == PackagePending) ||
		transcoded && (p.playsHLS() && prep.Ladder == PackagePending || prep.Legacy == PackagePending)
	switch {
	case directVideo && audioDefaultDirect(p, s):
		return Decision{Tier: TierDirect, Mode: "direct"}
	case pending:
		return Decision{Mode: "preparing"}
	case s.Available && srv.JIT && p.playsHLS() && transcoded:
		plan := jitPlan(p, s)
		return Decision{Tier: TierTranscode, Mode: "jit", JIT: &plan}
	}
	return Decision{Mode: "unsupported"}
}

// videoFit is whether the client decodes the source video as found in the
// file (direct) and as copied into the prepared Original rendition.
type videoFit struct{ direct, original bool }

func fitVideo(p DeviceProfile, s Source, srv Server) videoFit {
	v := s.Video
	if !p.video(s.VideoCodec).supports(v.Profile, v.Level, v.BitDepth) || !withinSize(p, s) {
		return videoFit{}
	}
	switch v.HDR {
	case media.HDR10, media.HDR10Plus:
		ok := p.hdr(media.HDR10) || p.hdr(media.HDR10Plus)
		return videoFit{direct: ok, original: ok && copyable(s.VideoCodec)}
	case media.HLG:
		ok := p.hdr(media.HLG)
		return videoFit{direct: ok, original: ok && copyable(s.VideoCodec)}
	case media.HDRDolbyVision:
		return fitDolbyVision(p, s, srv)
	}
	return videoFit{direct: true, original: copyable(s.VideoCodec)}
}

// fitDolbyVision plays Dolby Vision as such when the client decodes the
// profile, and otherwise falls back to the base layer: HDR10 for profiles 7 and
// 8.1, HLG for 8.4, SDR for 8.2. Profile 5 has no compatible base layer.
func fitDolbyVision(p DeviceProfile, s Source, srv Server) videoFit {
	v := s.Video
	asDovi := p.hdr(media.HDRDolbyVision + strconv.Itoa(v.DoviProfile))
	base := false
	switch v.DoviCompatibility {
	case 1, 6:
		base = p.hdr(media.HDR10) || p.hdr(media.HDR10Plus)
	case 4:
		base = p.hdr(media.HLG)
	case 2:
		base = true
	}
	fit := videoFit{}
	switch v.DoviProfile {
	case 5:
		// the prepared copy is only Dolby Vision when ffmpeg can say so
		fit.direct = asDovi
		fit.original = asDovi && srv.DolbyVisionCopy
	case 7:
		// the enhancement layer only belongs on clients that decode profile 7;
		// the prepared copy carries the HDR10 base layer alone
		fit.direct = asDovi
		fit.original = base
	default:
		fit.direct = asDovi || base
		fit.original = (asDovi && srv.DolbyVisionCopy) || base
	}
	fit.original = fit.original && copyable(s.VideoCodec)
	return fit
}

// copyable codecs can be copied into fMP4 HLS.
func copyable(codec string) bool {
	return codec == "h264" || codec == "hevc" || codec == "av1"
}

func withinSize(p DeviceProfile, s Source) bool {
	return (p.MaxWidth == 0 || s.Width <= p.MaxWidth) && (p.MaxHeight == 0 || s.Height <= p.MaxHeight) &&
		(p.MaxFrameRate == 0 || s.Video.FrameRate <= p.MaxFrameRate+0.01)
}

func withinBitrate(p DeviceProfile, bitrate int64) bool {
	return p.MaxBitrate == 0 || bitrate <= p.MaxBitrate
}

// containerName folds file-extension variants onto the profile vocabulary.
func containerName(c string) string {
	switch c {
	case "m4v":
		return "mp4"
	case "matroska":
		return "mkv"
	case "m2ts", "mts":
		return "ts"
	}
	return c
}

func defaultAudio(s Source) *media.AudioStream {
	for i := range s.Audio {
		if s.Audio[i].Default {
			return &s.Audio[i]
		}
	}
	if len(s.Audio) > 0 {
		return &s.Audio[0]
	}
	return nil
}

func audioDefaultDirect(p DeviceProfile, s Source) bool {
	a := defaultAudio(s)
	return a == nil || p.playsAudio(a.Codec, a.Channels)
}

// audioDirect needs every track playable when the client can switch between them.
func audioDirect(p DeviceProfile, s Source) bool {
	for _, a := range s.Audio {
		if !p.playsAudio(a.Codec, a.Channels) {
			return false
		}
	}
	return true
}

// needsRenditions is true when subtitles or alternate languages can only reach
// this client as HLS renditions.
func needsRenditions(p DeviceProfile, s Source) bool {
	if s.Subtitles > 0 && !slices.Contains(p.SidecarSubtitles, "webvtt") {
		return true
	}
	return len(s.Audio) > 1 && !p.AudioTrackSwitching
}

// surroundCodecs lists the multichannel codecs of the prepared surround
// renditions that the client takes.
func surroundCodecs(p DeviceProfile) []string {
	out := []string{}
	for _, c := range []string{"eac3", "ac3"} {
		if a := p.audio(c); a != nil && a.MaxChannels >= 6 {
			out = append(out, c)
		}
	}
	return out
}

// jitPlan always encodes the video: a copied stream would cut its segments at
// the source's keyframes, which are not known before ffmpeg reads them, and the
// session playlist lists every segment up front. The audio is copied when the
// client takes it.
func jitPlan(p DeviceProfile, s Source) JITPlan {
	plan := JITPlan{Video: "transcode", AudioStream: -1, Audio: "aac"}
	a := defaultAudio(s)
	if a == nil {
		return plan
	}
	plan.AudioStream = a.Index
	switch {
	case slices.Contains([]string{"aac", "ac3", "eac3"}, a.Codec) && p.playsAudio(a.Codec, a.Channels):
		plan.Audio = "copy"
	case a.Channels > 2 && slices.Contains(surroundCodecs(p), "eac3"):
		plan.Audio = "eac3"
	}
	return plan
}
