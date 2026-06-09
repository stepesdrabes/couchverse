package media

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type ProbeResult struct {
	Container       string
	DurationSeconds float64
	Bitrate         int64
	HasVideo        bool
	VideoCodec      string
	Width, Height   int
	VideoRange      string // sdr | hdr10 | hlg
	AudioCodec      string
	Channels        int
	SampleRate      int
	SubtitleStreams []SubtitleStream
	Raw             json.RawMessage
}

type SubtitleStream struct {
	Index    int
	Codec    string
	Language string
	Title    string
	Forced   bool
}

type probeOutput struct {
	Format struct {
		FormatName string `json:"format_name"`
		Duration   string `json:"duration"`
		BitRate    string `json:"bit_rate"`
	} `json:"format"`
	Streams []struct {
		Index         int    `json:"index"`
		CodecType     string `json:"codec_type"`
		CodecName     string `json:"codec_name"`
		Width         int    `json:"width"`
		Height        int    `json:"height"`
		ColorTransfer string `json:"color_transfer"`
		Channels      int    `json:"channels"`
		SampleRate    string `json:"sample_rate"`
		Disposition   struct {
			AttachedPic int `json:"attached_pic"`
			Forced      int `json:"forced"`
		} `json:"disposition"`
		Tags struct {
			Language string `json:"language"`
			Title    string `json:"title"`
		} `json:"tags"`
	} `json:"streams"`
}

// Probe runs ffprobe and extracts the technical metadata Couchverse needs.
func Probe(ctx context.Context, ffprobePath, file string) (*ProbeResult, error) {
	out, err := exec.CommandContext(ctx, ffprobePath,
		"-v", "quiet", "-print_format", "json", "-show_format", "-show_streams", file).Output()
	if err != nil {
		var exitErr *exec.ExitError
		if ok := isExitError(err, &exitErr); ok {
			return nil, fmt.Errorf("ffprobe: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("ffprobe: %w", err)
	}

	var parsed probeOutput
	if err := json.Unmarshal(out, &parsed); err != nil {
		return nil, fmt.Errorf("parse ffprobe output: %w", err)
	}

	res := &ProbeResult{
		Container:  containerName(parsed.Format.FormatName, file),
		VideoRange: "sdr",
		Raw:        out,
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
			switch s.ColorTransfer {
			case "smpte2084":
				res.VideoRange = "hdr10"
			case "arib-std-b67":
				res.VideoRange = "hlg"
			}
		case "audio":
			if res.AudioCodec != "" {
				continue
			}
			res.AudioCodec = s.CodecName
			res.Channels = s.Channels
			res.SampleRate, _ = strconv.Atoi(s.SampleRate)
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

func isExitError(err error, target **exec.ExitError) bool {
	if e, ok := err.(*exec.ExitError); ok {
		*target = e
		return true
	}
	return false
}
