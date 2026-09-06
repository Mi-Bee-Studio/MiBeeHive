//go:build windows

package diskutil

import (
	"fmt"

	"golang.org/x/sys/windows"
)

func Usage(path string) (total, free, avail uint64, err error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("resolving path %s: %w", path, err)
	}

	var availToCaller, totalBytes, totalFree uint64
	if err = windows.GetDiskFreeSpaceEx(p, &availToCaller, &totalBytes, &totalFree); err != nil {
		return 0, 0, 0, fmt.Errorf("GetDiskFreeSpaceEx %s: %w", path, err)
	}
	return totalBytes, totalFree, availToCaller, nil
}
