package playback

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"couchverse/internal/media"
)

// Run executes ffmpeg with args, reporting progress as a percentage of
// durationSeconds. background runs it below the web server's CPU and I/O priority
// (media.BackgroundCmd).
func Run(ctx context.Context, ffmpegPath string, args []string, background bool, durationSeconds float64, report func(pct int)) error {
	args = append([]string{"-progress", "pipe:1", "-nostats"}, args...)
	var cmd *exec.Cmd
	if background {
		cmd = media.BackgroundCmd(ctx, ffmpegPath, args...)
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
		if !strings.HasPrefix(line, "out_time_us=") || durationSeconds <= 0 || report == nil {
			continue
		}
		us, err := strconv.ParseInt(strings.TrimPrefix(line, "out_time_us="), 10, 64)
		if err != nil {
			continue
		}
		report(min(int(float64(us)/1e6/durationSeconds*100), 99))
	}

	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("ffmpeg: %s", strings.TrimSpace(stderr.String()))
	}
	return nil
}
