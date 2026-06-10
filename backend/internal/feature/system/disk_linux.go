//go:build linux

package system

import "syscall"

// diskUsage reports total and user-available bytes for the filesystem holding
// path. f_frsize is the fundamental block size that f_blocks is counted in;
// f_bsize is only the preferred I/O size and gets inflated on FUSE mounts
// (Docker Desktop reports 1 MiB), so - like df - we use f_frsize.
func diskUsage(path string) (total, free int64, ok bool) {
	var s syscall.Statfs_t
	if err := syscall.Statfs(path, &s); err != nil {
		return 0, 0, false
	}
	bs := s.Frsize
	if bs <= 0 {
		bs = s.Bsize
	}
	return int64(s.Blocks) * bs, int64(s.Bavail) * bs, true
}
