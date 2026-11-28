//go:build !darwin

package runner

// SystemFiles knows no limit off macOS, so the file-pressure preflight passes.
func SystemFiles() (uint32, uint32, error) { return 0, 0, nil }
