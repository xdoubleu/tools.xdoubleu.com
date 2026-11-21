package vm_test

import (
	"os"
	"testing"
)

// TestMain keeps the host's job environment out of the tests: a job VM's variables
// would leak into child processes.
func TestMain(m *testing.M) {
	for _, k := range []string{
		"FACTORY_PROXY", "FACTORY_GATEWAY", "FACTORY_JOB", "ANTHROPIC_BASE_URL",
	} {
		_ = os.Unsetenv(k)
	}
	os.Exit(m.Run())
}
