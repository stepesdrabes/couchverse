package media

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// ProbeVersion is bumped whenever Probe learns something new about a file;
// files probed by an older version are re-probed in the background.
const ProbeVersion = 2

// HDR formats of a video stream (media_files.hdr_format).
const (
	HDRNone        = "sdr"
	HDR10          = "hdr10"
	HDR10Plus      = "hdr10plus"
	HLG            = "hlg"
	HDRDolbyVision = "dolbyVision"
)

type ProbeResult struct {
	Container       string
	DurationSeconds float64
	Bitrate         int64
	HasVideo        bool
	Video           VideoStream
	VideoCodec      string
	Width, Height   int
	VideoRange      string // sdr | hdr10 | hlg | dv
	AudioCodec      string
	Channels        int
	SampleRate      int
	AudioStreams    []AudioStream
	SubtitleStreams []SubtitleStream
	Raw             json.RawMessage
}

// VideoStream is what playback decisions need to know about a file's video.
type VideoStream struct {
	CodecTag          string  `json:"codecTag" doc:"The container's codec tag (hvc1, hev1, dvh1, avc1); empty when the container has none (Matroska)."`
	Profile           string  `json:"profile" doc:"Normalized codec profile: baseline, main, high, high10, main10, rext..."`
	Level             float64 `json:"level" doc:"Codec level as written, e.g. 4.1 or 5.1; 0 when unknown."`
	BitDepth          int     `json:"bitDepth"`
	FrameRate         float64 `json:"frameRate"`
	HDR               string  `json:"hdr" enum:"sdr,hdr10,hdr10plus,hlg,dolbyVision"`
	DoviProfile       int     `json:"doviProfile" doc:"Dolby Vision profile (5, 7, 8); 0 without Dolby Vision."`
	DoviCompatibility int     `json:"doviCompatibility" doc:"Dolby Vision base-layer compatibility id: 1 HDR10, 2 SDR, 4 HLG, 6 Blu-ray HDR10, 0 none."`
}

type SubtitleStream struct {
	Index    int
	Codec    string
	Language string
	Title    string
	Forced   bool
}

type probeStream struct {
	Index         int    `json:"index"`
	CodecType     string `json:"codec_type"`
	CodecName     string `json:"codec_name"`
	CodecTag      string `json:"codec_tag_string"`
	Profile       string `json:"profile"`
	Level         int    `json:"level"`
	PixFmt        string `json:"pix_fmt"`
	RawBits       string `json:"bits_per_raw_sample"`
	RFrameRate    string `json:"r_frame_rate"`
	AvgFrameRate  string `json:"avg_frame_rate"`
	Width         int    `json:"width"`
	Height        int    `json:"height"`
	ColorTransfer string `json:"color_transfer"`
	Channels      int    `json:"channels"`
	ChannelLayout string `json:"channel_layout"`
	SampleRate    string `json:"sample_rate"`
	SideData      []struct {
		Type          string `json:"side_data_type"`
		DVProfile     int    `json:"dv_profile"`
		Compatibility int    `json:"dv_bl_signal_compatibility_id"`
	} `json:"side_data_list"`
	Disposition struct {
		AttachedPic int `json:"attached_pic"`
		Forced      int `json:"forced"`
		Default     int `json:"default"`
	} `json:"disposition"`
	Tags struct {
		Language string `json:"language"`
		Title    string `json:"title"`
	} `json:"tags"`
}

type probeOutput struct {
	Format struct {
		FormatName string `json:"format_name"`
		Duration   string `json:"duration"`
		BitRate    string `json:"bit_rate"`
	} `json:"format"`
	Streams []probeStream `json:"streams"`
}

// Probe runs ffprobe and extracts the technical metadata Couchverse needs.
func Probe(ctx context.Context, ffprobePath, file string) (*ProbeResult, error) {
	out, err := exec.CommandContext(ctx, ffprobePath,
		"-v", "quiet", "-print_format", "json", "-show_format", "-show_streams", file).Output()
	if err != nil {
		return nil, ffprobeError(err)
	}
	res, err := ParseProbe(out, file)
	if err != nil {
		return nil, err
	}
	// HDR10+ metadata travels per frame (SEI), so only a decoded frame shows it
	if res.Video.HDR == HDR10 && hasHDR10Plus(ctx, ffprobePath, file) {
		res.Video.HDR = HDR10Plus
	}
	return res, nil
}

func ffprobeError(err error) error {
	if exitErr, ok := err.(*exec.ExitError); ok {
		return fmt.Errorf("ffprobe: %s", strings.TrimSpace(string(exitErr.Stderr)))
	}
	return fmt.Errorf("ffprobe: %w", err)
}

// hasHDR10Plus reads the first video frame's side data.
func hasHDR10Plus(ctx context.Context, ffprobePath, file string) bool {
	out, err := exec.CommandContext(ctx, ffprobePath, "-v", "quiet", "-select_streams", "v:0",
		"-read_intervals", "%+#1", "-show_frames", "-show_entries", "frame=side_data_list",
		"-print_format", "json", file).Output()
	return err == nil && strings.Contains(string(out), "SMPTE2094-40")
}

// ParseProbe reads ffprobe's -show_format -show_streams JSON. The stored probe
// of a file whose source is gone can be parsed again this way.
func ParseProbe(raw []byte, file string) (*ProbeResult, error) {
	var parsed probeOutput
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("parse ffprobe output: %w", err)
	}

	res := &ProbeResult{
		Container:  containerName(parsed.Format.FormatName, file),
		VideoRange: "sdr",
		Raw:        raw,
	}
	res.DurationSeconds, _ = strconv.ParseFloat(parsed.Format.Duration, 64)
	res.Bitrate, _ = strconv.ParseInt(parsed.Format.BitRate, 10, 64)

	for _, s := range parsed.Streams {
		switch s.CodecType {
		case "video":
			// embedded cover art shows up as an attached_pic video stream
			if s.Disposition.AttachedPic == 1 || res.HasVideo {
				continue
			}
			res.HasVideo = true
			res.VideoCodec = s.CodecName
			res.Width, res.Height = s.Width, s.Height
			res.Video = videoStream(s)
			switch res.Video.HDR {
			case HDR10, HDR10Plus:
				res.VideoRange = "hdr10"
			case HLG:
				res.VideoRange = "hlg"
			case HDRDolbyVision:
				res.VideoRange = "dv"
			}
		case "audio":
			sr, _ := strconv.Atoi(s.SampleRate)
			res.AudioStreams = append(res.AudioStreams, AudioStream{
				Index: s.Index, Codec: s.CodecName, Lang: s.Tags.Language,
				Title: s.Tags.Title, Channels: s.Channels, Default: s.Disposition.Default == 1,
				Profile: knownProfile(s.Profile), ChannelLayout: s.ChannelLayout, SampleRate: sr,
			})
			if res.AudioCodec != "" {
				continue
			}
			res.AudioCodec = s.CodecName
			res.Channels = s.Channels
			res.SampleRate = sr
		case "subtitle":
			res.SubtitleStreams = append(res.SubtitleStreams, SubtitleStream{
				Index:    s.Index,
				Codec:    s.CodecName,
				Language: s.Tags.Language,
				Title:    s.Tags.Title,
				Forced:   s.Disposition.Forced == 1,
			})
		}
	}
	return res, nil
}

func videoStream(s probeStream) VideoStream {
	v := VideoStream{
		CodecTag:  codecTag(s.CodecTag),
		Profile:   normalizeProfile(s.Profile),
		Level:     normalizeLevel(s.CodecName, s.Level),
		BitDepth:  bitDepth(s.PixFmt, s.RawBits),
		FrameRate: frameRate(s.AvgFrameRate),
		HDR:       HDRNone,
	}
	if v.FrameRate == 0 {
		v.FrameRate = frameRate(s.RFrameRate)
	}
	switch s.ColorTransfer {
	case "smpte2084":
		v.HDR = HDR10
	case "arib-std-b67":
		v.HDR = HLG
	}
	for _, sd := range s.SideData {
		if sd.Type == "DOVI configuration record" {
			v.HDR = HDRDolbyVision
			v.DoviProfile, v.DoviCompatibility = sd.DVProfile, sd.Compatibility
		}
	}
	return v
}

// codecTag drops ffprobe's placeholder for containers without codec tags.
func codecTag(tag string) string {
	if strings.HasPrefix(tag, "[") || strings.HasPrefix(tag, "0x") {
		return ""
	}
	return tag
}

func knownProfile(p string) string {
	if p == "unknown" {
		return ""
	}
	return p
}

// normalizeProfile turns ffprobe's profile names into the capability
// vocabulary: "Main 10" -> main10, "Constrained Baseline" -> baseline,
// "High 10" -> high10, "Profile 2" -> profile2.
func normalizeProfile(p string) string {
	p = strings.ToLower(strings.TrimSpace(knownProfile(p)))
	p = strings.TrimPrefix(p, "constrained ")
	p = strings.TrimSuffix(p, " intra")
	switch p {
	case "high 4:2:2":
		return "high422"
	case "high 4:4:4 predictive", "high 4:4:4":
		return "high444"
	}
	return strings.ReplaceAll(p, " ", "")
}

// normalizeLevel converts ffprobe's integer level to the written one:
// H.264 stores 41 for 4.1, HEVC 30 times the level, AV1 its seq_level_idx.
func normalizeLevel(codec string, level int) float64 {
	if level <= 0 {
		return 0
	}
	switch codec {
	case "h264":
		return float64(level) / 10
	case "hevc":
		return float64(level*10/30) / 10
	case "av1":
		return float64(2+level/4) + float64(level%4)/10
	}
	return 0
}

func bitDepth(pixFmt, raw string) int {
	if n, err := strconv.Atoi(raw); err == nil && n > 0 {
		return n
	}
	for _, depth := range []int{16, 14, 12, 10, 9} {
		if strings.Contains(pixFmt, "p"+strconv.Itoa(depth)) {
			return depth
		}
	}
	if pixFmt == "" {
		return 0
	}
	return 8
}

// frameRate evaluates ffprobe's "num/den" rates.
func frameRate(s string) float64 {
	num, den, ok := strings.Cut(s, "/")
	n, err1 := strconv.ParseFloat(num, 64)
	d, err2 := strconv.ParseFloat(den, 64)
	if !ok || err1 != nil || err2 != nil || d == 0 {
		return 0
	}
	return n / d
}

// containerName normalizes ffprobe's comma-separated format list using the
// file extension as the tiebreaker (e.g. "mov,mp4,m4a,3gp,3g2,mj2" → "mp4").
func containerName(formatName, file string) string {
	ext := strings.TrimPrefix(strings.ToLower(fileExt(file)), ".")
	options := strings.Split(formatName, ",")
	for _, o := range options {
		if o == ext {
			return o
		}
	}
	if ext == "mkv" || formatName == "matroska,webm" {
		if ext == "webm" {
			return "webm"
		}
		return "mkv"
	}
	if ext != "" {
		return ext
	}
	return options[0]
}

func fileExt(file string) string {
	i := strings.LastIndex(file, ".")
	if i < 0 {
		return ""
	}
	return file[i:]
}
