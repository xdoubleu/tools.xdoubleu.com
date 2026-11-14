package runner_test

import (
	"os"
	"path/filepath"
	"testing"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

// TestMain keeps every test off the real /Applications.
func TestMain(m *testing.M) {
	runner.ApplicationsDir = func(home string) string {
		return filepath.Join(home, "Library", "Applications")
	}
	os.Exit(m.Run())
}
