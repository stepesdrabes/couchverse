package transcode

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// BuildArgs assembles the ffmpeg command for one HLS variant.
// mode "copy" remuxes the h264 stream (I/O-bound, fast everywhere);
// mode "transcode" re-encodes through the chosen encoder.
type BuildSpec struct {
	Input          string
	OutDir         string // segments + index.m3u8 land here
	Mode           string // copy | transcode
	Rendition      Rendition
	Encoder        string // libx264 | h264_videotoolbox | ...
	Preset         string
	HasAudio       bool
	StartAt        float64 // JIT sessions seek before encoding
	JIT            bool    // live session: timestamp offset, atomic segments, no VOD playlist
	StartNumber    int     // first segment index (JIT restarts)
	BackgroundNice bool
}

func BuildArgs(spec BuildSpec) []string {
	args := []string{"-hide_banner", "-loglevel", "error", "-y"}
	if spec.StartAt > 0 {
		args = append(args, "-ss", fmt.Sprintf("%.3f", spec.StartAt))
	}
	args = append(args, "-i", spec.Input, "-map", "0:v:0")
	if spec.HasAudio {
		args = append(args, "-map", "0:a:0")
	}

	if spec.Mode == "copy" {
		args = append(args, "-c:v", "copy")
	} else {
		r := spec.Rendition
		args = append(args, "-vf", fmt.Sprintf("scale=-2:%d", r.Height))
		switch spec.Encoder {
		case "libx264":
			args = append(args, "-c:v", "libx264",
				"-preset", spec.Preset,
				"-crf", "23",
				"-maxrate", strconv.FormatInt(r.VideoBitrate, 10),
				"-bufsize", strconv.FormatInt(2*r.VideoBitrate, 10))
		default: // hardware encoders take a target bitrate
			args = append(args, "-c:v", spec.Encoder,
				"-b:v", strconv.FormatInt(r.VideoBitrate, 10),
				"-maxrate", strconv.FormatInt(r.VideoBitrate, 10),
				"-bufsize", strconv.FormatInt(2*r.VideoBitrate, 10))
		}
		args = append(args, "-force_key_frames", "expr:gte(t,n_forced*4)")
	}

	if spec.HasAudio {
		audioBitrate := spec.Rendition.AudioBitrate
		if audioBitrate == 0 {
			audioBitrate = 192_000
		}
		args = append(args,
			"-c:a", "aac",
			"-b:a", strconv.FormatInt(audioBitrate, 10),
			"-ac", "2")
	}

	args = append(args, "-f", "hls", "-hls_time", "4")
	if spec.JIT {
		// the session playlist is generated in Go; ffmpeg's own playlist is
		// internal. temp_file makes finished segments appear atomically and
		// the ts offset keeps timestamps aligned with the virtual timeline.
		args = append(args,
			"-output_ts_offset", fmt.Sprintf("%.3f", spec.StartAt),
			"-start_number", strconv.Itoa(spec.StartNumber),
			"-hls_flags", "temp_file",
			"-hls_list_size", "0",
		)
	} else {
		args = append(args, "-hls_playlist_type", "vod")
	}
	args = append(args,
		"-hls_segment_filename", spec.OutDir+"/seg_%05d.ts",
		"-progress", "pipe:1",
		spec.OutDir+"/index.m3u8",
	)
	return args
}

// Run executes ffmpeg, reporting progress as a percentage of durationSeconds.
func Run(ctx context.Context, ffmpegPath string, spec BuildSpec, durationSeconds float64, report func(pct int)) error {
	args := BuildArgs(spec)

	var cmd *exec.Cmd
	if spec.BackgroundNice {
		cmd = exec.CommandContext(ctx, "nice", append([]string{"-n", "19", ffmpegPath}, args...)...)
	} else {
		cmd = exec.CommandContext(ctx, ffmpegPath, args...)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr // -loglevel error keeps this small

	if err := cmd.Start(); err != nil {
		return err
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "out_time_us=") || durationSeconds <= 0 {
			continue
		}
		us, err := strconv.ParseInt(strings.TrimPrefix(line, "out_time_us="), 10, 64)
		if err != nil {
			continue
		}
		pct := int(float64(us) / 1e6 / durationSeconds * 100)
		if report != nil {
			report(min(pct, 99))
		}
	}

	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("ffmpeg: %s", strings.TrimSpace(stderr.String()))
	}
	return nil
}
