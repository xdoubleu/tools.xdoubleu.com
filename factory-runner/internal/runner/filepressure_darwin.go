//go:build darwin

package runner

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// SystemFiles is how many files are open on this Mac and its limit.
func SystemFiles() (uint32, uint32, error) {
	open, err := unix.SysctlUint32("kern.num_files")
	if err != nil {
		return 0, 0, fmt.Errorf("kern.num_files: %w", err)
	}
	limit, err := unix.SysctlUint32("kern.maxfiles")
	if err != nil {
		return 0, 0, fmt.Errorf("kern.maxfiles: %w", err)
	}
	return open, limit, nil
}
