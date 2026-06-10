//go:build darwin

package system

import "syscall"

// diskUsage reports total and user-available bytes for the filesystem holding
// path. macOS statfs has no f_frsize; f_bsize is the real block size here.
func diskUsage(path string) (total, free int64, ok bool) {
	var s syscall.Statfs_t
	if err := syscall.Statfs(path, &s); err != nil {
		return 0, 0, false
	}
	bs := int64(s.Bsize)
	return int64(s.Blocks) * bs, int64(s.Bavail) * bs, true
}
