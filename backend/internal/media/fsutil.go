package media

import (
	"io/fs"
	"path/filepath"
)

// DirSize sums the sizes of all regular files under path. Missing or
// unreadable entries are skipped.
func DirSize(path string) int64 {
	var total int64
	_ = filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}
