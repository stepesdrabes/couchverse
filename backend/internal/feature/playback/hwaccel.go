// Package playback serves video: direct play, prepared HLS variants and
// just-in-time transcode sessions, with hardware acceleration when the host
// has a usable encoder.
package playback

import (
	"context"
	"log/slog"
	"os/exec"
	"sync"
	"time"

	"couchverse/internal/media"
)

var (
	detectOnce sync.Once
	detectDone bool
	detectMu   sync.Mutex
	detected   []string
)

// DetectEncoders probes which hardware h264 encoders actually work by running
// a tiny test encode. Results are cached for the process lifetime.
func DetectEncoders(ffmpegPath string) []string {
	detectOnce.Do(func() {
		var found []string
		for _, encoder := range media.HWEncoderCandidates {
			if testEncode(ffmpegPath, encoder) {
				found = append(found, encoder)
			}
		}
		detectMu.Lock()
		detected = found
		detectDone = true
		detectMu.Unlock()
		slog.Info("hardware encoders detected", "encoders", found)
	})
	return detected
}

// DetectedEncoders returns the probe results so far without blocking on the
// probe itself; done is false while detection is still running.
func DetectedEncoders() (encoders []string, done bool) {
	detectMu.Lock()
	defer detectMu.Unlock()
	return detected, detectDone
}

func testEncode(ffmpegPath, encoder string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	args := []string{
		"-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "testsrc2=size=320x180:rate=25:duration=0.2",
	}
	if encoder == "h264_vaapi" {
		args = append(args, "-vaapi_device", "/dev/dri/renderD128", "-vf", "format=nv12,hwupload")
	}
	args = append(args, "-c:v", encoder, "-f", "null", "-")

	return exec.CommandContext(ctx, ffmpegPath, args...).Run() == nil
}

// PickEncoder resolves the transcode.hw_accel setting to a concrete encoder.
// "auto" → best detected hw encoder, falling back to software libx264.
func PickEncoder(ffmpegPath, setting string) string {
	encoders := DetectEncoders(ffmpegPath)
	switch setting {
	case "", "auto":
		if len(encoders) > 0 {
			return encoders[0]
		}
		return "libx264"
	case "none":
		return "libx264"
	default:
		for _, e := range encoders {
			if e == setting {
				return e
			}
		}
		return "libx264"
	}
}
