package media

import (
	"context"
	"os/exec"
	"sync"
)

// hasIonice reports whether ionice is installed (it is on Linux, not on macOS).
var hasIonice = sync.OnceValue(func() bool {
	_, err := exec.LookPath("ionice")
	return err == nil
})

// BackgroundCmd runs name as background work on a small server: at the lowest CPU
// priority and, where ionice exists, in the idle I/O class. On a Pi the disks are the
// bottleneck long before the cores are, and the web server and the live stream read
// the same disks.
func BackgroundCmd(ctx context.Context, name string, args ...string) *exec.Cmd {
	wrap := []string{"-n", "19"}
	if hasIonice() {
		wrap = append(wrap, "ionice", "-c", "3")
	}
	return exec.CommandContext(ctx, "nice", append(append(wrap, name), args...)...)
}
