//go:build darwin

package runner

import "golang.org/x/sys/unix"

// SystemFiles is how many files are open on this Mac and its limit.
func SystemFiles() (uint32, uint32, error) {
	return systemFiles(unix.SysctlUint32)
}
