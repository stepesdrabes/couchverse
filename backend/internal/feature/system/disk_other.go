//go:build !linux && !darwin

package system

// diskUsage is unsupported on this platform; the storage meter hides itself
// when disk stats are unavailable.
func diskUsage(path string) (total, free int64, ok bool) {
	return 0, 0, false
}
