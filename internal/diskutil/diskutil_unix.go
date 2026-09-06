//go:build unix

package diskutil

import (
	"fmt"
	"syscall"
)

func Usage(path string) (total, free, avail uint64, err error) {
	var stat syscall.Statfs_t
	if err = syscall.Statfs(path, &stat); err != nil {
		return 0, 0, 0, fmt.Errorf("statfs %s: %w", path, err)
	}

	bs := uint64(stat.Bsize)
	total = uint64(stat.Blocks) * bs
	free = uint64(stat.Bfree) * bs
	avail = uint64(stat.Bavail) * bs
	return total, free, avail, nil
}
