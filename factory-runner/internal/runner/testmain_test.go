package runner_test

import (
	"os"
	"path/filepath"
	"testing"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

// TestMain keeps every test off the real /Applications and out of the host's
// job environment.
func TestMain(m *testing.M) {
	runner.ApplicationsDir = func(home string) string {
		return filepath.Join(home, "Library", "Applications")
	}
	// Variables a job VM sets would leak into child processes (proxy, git identity).
	for _, k := range []string{
		"FACTORY_PROXY", "FACTORY_GATEWAY", "FACTORY_JOB", "ANTHROPIC_BASE_URL",
		"GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL", "GIT_COMMITTER_NAME",
		"GIT_COMMITTER_EMAIL",
	} {
		_ = os.Unsetenv(k)
	}
	os.Exit(m.Run())
}
