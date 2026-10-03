package playback

import (
	"context"
	"log/slog"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Features is what the installed ffmpeg can do beyond the basics.
type Features struct {
	// Major is ffmpeg's major version; 0 for a git build without one.
	Major int
	// DolbyVision: the mp4 muxer tags dvh1 and writes dvcC/dvvC (ffmpeg 6+),
	// so copied Dolby Vision stays Dolby Vision.
	DolbyVision bool
	// ToneMap: zscale and tonemap are built in, so HDR becomes proper SDR.
	ToneMap bool
}

var (
	featuresOnce sync.Once
	features     Features
)

var versionRe = regexp.MustCompile(`ffmpeg version n?(\d+)\.`)

// DetectFeatures inspects ffmpeg once per process.
func DetectFeatures(ffmpegPath string) Features {
	featuresOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if out, err := exec.CommandContext(ctx, ffmpegPath, "-hide_banner", "-version").Output(); err == nil {
			if m := versionRe.FindSubmatch(out); m != nil {
				features.Major, _ = strconv.Atoi(string(m[1]))
			}
		}
		// git builds report a commit instead of a version; they are recent
		features.DolbyVision = features.Major == 0 || features.Major >= 6
		if out, err := exec.CommandContext(ctx, ffmpegPath, "-hide_banner", "-filters").Output(); err == nil {
			features.ToneMap = hasFilter(string(out), "zscale") && hasFilter(string(out), "tonemap")
		}
		slog.Info("ffmpeg features", "major", features.Major, "dolbyVision", features.DolbyVision, "toneMap", features.ToneMap)
		if !features.ToneMap {
			slog.Warn("ffmpeg lacks zscale/tonemap: HDR sources transcode to SDR without tone mapping")
		}
	})
	return features
}

func hasFilter(list, name string) bool {
	for _, line := range strings.Split(list, "\n") {
		if f := strings.Fields(line); len(f) >= 2 && f[1] == name {
			return true
		}
	}
	return false
}
