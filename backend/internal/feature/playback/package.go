package playback

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/text/language"

	"couchverse/internal/hls"
	"couchverse/internal/media"
	"couchverse/internal/media/mp4"
)

// Every rendition of a file is cut from one timeline: the input keeps its own
// timestamps (-copyts, from zero) and every output starts 1.4 s in, the MPEG-TS
// convention players expect. The copied video's B-frame delay and the audio
// encoder's priming then never fall below zero, so ffmpeg never shifts one
// output against another, and WebVTT cues map onto the same timeline with
// X-TIMESTAMP-MAP=MPEGTS:126000.
const (
	timelineOffset = "1.4"
	subtitleMPEGTS = 126000
	// segmentSeconds is the HLS segment length; transcodes put an IDR every
	// keyframeSeconds so every rendition cuts at the same instants.
	segmentSeconds  = 6.0
	keyframeSeconds = 2
	// trickSeconds spaces the frames of the I-frame (trick-play) rendition.
	trickSeconds = 2
)

// hlsOutput is one ffmpeg output of a job: a directory with index.m3u8,
// init.mp4 and the fMP4 segments.
type hlsOutput struct {
	dir     string
	codec   []string // map and codec options
	video   bool
	segment float64
}

func (o hlsOutput) args() []string {
	args := append([]string{}, o.codec...)
	args = append(args,
		"-output_ts_offset", timelineOffset,
		"-f", "hls",
		"-hls_time", strconv.FormatFloat(o.segment, 'f', -1, 64),
		"-hls_playlist_type", "vod",
		"-hls_segment_type", "fmp4",
		// frag_discont keeps the real decode time in tfdt and negative CTS
		// offsets put the first video frame on it, so the timeline does not
		// depend on edit lists, which hls.js ignores
		"-hls_segment_options", "movflags=+frag_discont+negative_cts_offsets",
		"-hls_fmp4_init_filename", "init.mp4")
	if o.video {
		args = append(args, "-hls_flags", "independent_segments")
	}
	return append(args,
		"-hls_segment_filename", filepath.Join(o.dir, "seg_%05d.m4s"),
		filepath.Join(o.dir, "index.m3u8"))
}

// inputArgs open the source on the shared timeline.
func inputArgs(input string, startAt float64, extra ...string) []string {
	args := []string{"-hide_banner", "-loglevel", "error", "-y", "-copyts", "-start_at_zero"}
	if startAt > 0 {
		args = append(args, "-ss", fmt.Sprintf("%.3f", startAt))
	}
	args = append(args, extra...)
	return append(args, "-i", input)
}

// sourceVideoArgs copy the video stream into fMP4. HEVC is tagged hvc1, which
// AVPlayer requires. Dolby Vision keeps its signaling where ffmpeg can write
// it (profile 8 stays hvc1 so HDR10 players decode it; profile 5 has to be
// dvh1); profile 7 loses its enhancement layer and RPUs and plays as the HDR10
// base layer it carries.
func sourceVideoArgs(mf *media.MediaFile, f Features) []string {
	args := []string{"-map", "0:v:0", "-c:v", "copy"}
	v := mf.Video
	switch {
	case v.HDR == media.HDRDolbyVision && v.DoviProfile == 7:
		args = append(args, "-tag:v", "hvc1", "-bsf:v", "filter_units=remove_types=62|63")
	case v.HDR == media.HDRDolbyVision && v.DoviProfile == 5:
		args = append(args, "-tag:v", "dvh1", "-strict", "unofficial")
	case v.HDR == media.HDRDolbyVision && f.DolbyVision:
		args = append(args, "-tag:v", "hvc1", "-strict", "unofficial")
	case mf.VideoCodec == "hevc":
		args = append(args, "-tag:v", "hvc1")
	}
	return args
}

// canCopySource reports whether the source video goes into the package as is.
func canCopySource(mf *media.MediaFile, f Features) bool {
	if !copyable(mf.VideoCodec) || mf.Video.BitDepth > 10 {
		return false
	}
	// without dvh1 a copied profile 5 stream is unreadable to everyone
	return mf.Video.HDR != media.HDRDolbyVision || mf.Video.DoviProfile != 5 || f.DolbyVision
}

// audioRendition is one prepared audio stream of a source track.
type audioRendition struct {
	track media.AudioStream
	codec string // aac, ac3 or eac3
	copy  bool
}

func (a audioRendition) dir() string {
	return fmt.Sprintf("audio-%d-%s", a.track.Index, a.codec)
}

// audioRenditions plans the audio of a package: every track gets AAC stereo,
// and multichannel tracks also a surround rendition, copied when it already is
// AC-3 or E-AC-3 (Atmos included) and E-AC-3 5.1 otherwise (DTS, TrueHD, PCM...).
func audioRenditions(tracks []media.AudioStream) []audioRendition {
	var out []audioRendition
	for _, t := range tracks {
		out = append(out, audioRendition{track: t, codec: "aac", copy: t.Codec == "aac" && t.Channels <= 2})
		if t.Channels > 2 {
			switch t.Codec {
			case "ac3", "eac3":
				out = append(out, audioRendition{track: t, codec: t.Codec, copy: true})
			default:
				out = append(out, audioRendition{track: t, codec: "eac3"})
			}
		}
	}
	return out
}

func (a audioRendition) args() []string {
	args := []string{"-map", fmt.Sprintf("0:%d", a.track.Index)}
	switch {
	case a.copy:
		return append(args, "-c:a", "copy")
	case a.codec == "aac":
		return append(args, "-c:a", "aac", "-ac", "2", "-ar", "48000", "-b:a", "160k")
	}
	return append(args, "-c:a", "eac3", "-ac", "6", "-ar", "48000", "-b:a", "640k")
}

// toneMapFilter turns HDR into BT.709 SDR; ffmpeg builds without zscale fall
// back to a plain 8-bit conversion (washed out, but playable).
func toneMapFilter(f Features) string {
	if !f.ToneMap {
		return "setparams=color_primaries=bt709:color_trc=bt709:colorspace=bt709,format=yuv420p"
	}
	return "zscale=t=linear:npl=100,format=gbrpf32le,zscale=p=bt709," +
		"tonemap=tonemap=hable:desat=0,zscale=t=bt709:m=bt709:r=tv,format=yuv420p"
}

func isHDR(v media.VideoStream) bool {
	// a Dolby Vision 8.2 base layer is SDR
	return v.HDR != media.HDRNone && v.HDR != "" && (v.HDR != media.HDRDolbyVision || v.DoviCompatibility != 2)
}

// videoFilter scales to height and always ends in 8-bit 4:2:0, so 10-bit and
// HDR sources come out as H.264 every player decodes.
func videoFilter(mf *media.MediaFile, height int, f Features) string {
	filter := fmt.Sprintf("scale=-2:%d", height)
	if isHDR(mf.Video) {
		return filter + "," + toneMapFilter(f)
	}
	return filter + ",format=yuv420p"
}

// sdrTags label the transcoded video BT.709 in the bitstream and the init.
var sdrTags = []string{"-color_primaries", "bt709", "-color_trc", "bt709", "-colorspace", "bt709"}

// rungVideoArgs encode one ladder rung: H.264 SDR with an IDR every two
// seconds of source time, so every rung cuts its segments at the same instants.
func rungVideoArgs(mf *media.MediaFile, r media.Rendition, encoder, preset string, f Features, startAt float64) []string {
	args := []string{"-map", "0:v:0", "-vf", videoFilter(mf, r.Height, f)}
	args = append(args, encoderArgs(encoder, preset, r.VideoBitrate)...)
	gop := int(max(mf.Video.FrameRate, 1)*keyframeSeconds + 0.5)
	args = append(args, "-g", strconv.Itoa(gop),
		"-force_key_frames", fmt.Sprintf("expr:gte(t,%g+n_forced*%d)", startAt, keyframeSeconds))
	return append(args, sdrTags...)
}

func encoderArgs(encoder, preset string, bitrate int64) []string {
	rate := strconv.FormatInt(bitrate, 10)
	buf := strconv.FormatInt(2*bitrate, 10)
	if encoder == "libx264" {
		return []string{"-c:v", "libx264", "-preset", preset, "-profile:v", "high", "-crf", "23",
			"-maxrate", rate, "-bufsize", buf}
	}
	// hardware encoders take a target bitrate
	return []string{"-c:v", encoder, "-profile:v", "high", "-b:v", rate, "-maxrate", rate, "-bufsize", buf}
}

// trickplayArgs make the I-frame rendition: one small intra frame every
// trickSeconds, decoded from the source's keyframes only so it stays cheap
// even for 4K HEVC on a Pi.
func trickplayArgs(mf *media.MediaFile, f Features) []string {
	filter := fmt.Sprintf("fps=1/%d,scale=-2:180", trickSeconds)
	if isHDR(mf.Video) {
		filter += "," + toneMapFilter(f)
	} else {
		filter += ",format=yuv420p"
	}
	args := []string{"-map", "0:v:0", "-vf", filter, "-c:v", "libx264", "-preset", "veryfast",
		"-profile:v", "high", "-crf", "30", "-g", "1"}
	return append(args, sdrTags...)
}

// renditionInfo describes a finished rendition directory (rendition.json), so
// multivariant playlists are written without opening media files.
type renditionInfo struct {
	Codec             string  `json:"codec"`
	SupplementalCodec string  `json:"supplementalCodec,omitempty"`
	Width             int     `json:"width,omitempty"`
	Height            int     `json:"height,omitempty"`
	FrameRate         float64 `json:"frameRate,omitempty"`
	VideoRange        string  `json:"videoRange,omitempty"`
	Channels          string  `json:"channels,omitempty"`
	StreamIndex       int     `json:"streamIndex"`
	Language          string  `json:"language,omitempty"`
	Name              string  `json:"name,omitempty"`
	Default           bool    `json:"default,omitempty"`
	Peak              int64   `json:"peak"`
	Average           int64   `json:"average"`
	Duration          float64 `json:"duration"`
}

const renditionFile = "rendition.json"

// describeRendition measures a finished rendition directory and writes its
// rendition.json; base carries what the media cannot tell (language, name...).
func describeRendition(dir string, base renditionInfo) (renditionInfo, error) {
	text, err := os.ReadFile(filepath.Join(dir, "index.m3u8"))
	if err != nil {
		return base, err
	}
	pl, err := hls.Parse(string(text))
	if err != nil || pl.Media == nil || len(pl.Media.Segments) == 0 {
		return base, fmt.Errorf("%s: not a media playlist", dir)
	}
	initData, err := os.ReadFile(filepath.Join(dir, "init.mp4"))
	if err != nil {
		return base, err
	}
	in, err := mp4.ParseInit(initData)
	if err != nil {
		return base, err
	}
	t := in.Tracks[0]
	info := base
	info.Codec, info.SupplementalCodec = t.Codec, t.SupplementalCodec
	if t.Handler == "vide" {
		info.Width, info.Height = t.Width, t.Height
	} else {
		info.Channels = strconv.Itoa(t.Channels)
		if t.JOC {
			info.Channels = "16/JOC"
		}
	}
	var durations, sizes []float64
	for _, s := range pl.Media.Segments {
		st, err := os.Stat(filepath.Join(dir, s.URI))
		if err != nil {
			return base, err
		}
		durations = append(durations, s.Duration)
		sizes = append(sizes, float64(st.Size()))
		info.Duration += s.Duration
	}
	peak, average := hls.Bitrates(durations, sizes, pl.Media.TargetDuration)
	info.Peak, info.Average = int64(peak+0.5), int64(average+0.5)
	data, err := json.Marshal(info)
	if err != nil {
		return info, err
	}
	return info, os.WriteFile(filepath.Join(dir, renditionFile), data, 0o644)
}

func readRendition(dir string) (renditionInfo, error) {
	var info renditionInfo
	data, err := os.ReadFile(filepath.Join(dir, renditionFile))
	if err != nil {
		return info, err
	}
	return info, json.Unmarshal(data, &info)
}

// writeIFramePlaylist lists the trick-play segments as I-frames: each one holds
// a single intra frame, so the whole segment is the I-frame.
func writeIFramePlaylist(dir string) error {
	text, err := os.ReadFile(filepath.Join(dir, "index.m3u8"))
	if err != nil {
		return err
	}
	pl, err := hls.Parse(string(text))
	if err != nil || pl.Media == nil {
		return fmt.Errorf("%s: not a media playlist", dir)
	}
	m := pl.Media
	m.IFramesOnly, m.IndependentSegments, m.PlaylistType = true, true, "VOD"
	return os.WriteFile(filepath.Join(dir, "iframes.m3u8"), []byte(m.String()), 0o644)
}

// videoRange is the VIDEO-RANGE of the source video as copied: Dolby Vision
// reports its base layer, profile 5 (IPT, PQ-based) PQ.
func videoRange(v media.VideoStream) string {
	switch v.HDR {
	case media.HDR10, media.HDR10Plus:
		return "PQ"
	case media.HLG:
		return "HLG"
	case media.HDRDolbyVision:
		switch v.DoviCompatibility {
		case 2:
			return "SDR"
		case 4:
			return "HLG"
		}
		return "PQ"
	}
	return "SDR"
}

// bcp47 turns a stream's ISO 639-2 tag (eng, cze) into the BCP 47 form HLS
// wants (en, cs); unknown tags become und.
func bcp47(lang string) string {
	if lang == "" {
		return "und"
	}
	tag, err := language.Parse(lang)
	if err != nil {
		return "und"
	}
	base, conf := tag.Base()
	if conf != language.Exact {
		return "und"
	}
	return strings.ToLower(base.String())
}
