package vm_test

import (
	"os"
	"testing"
)

// TestMain keeps the host's job environment out of the tests: a job VM's variables
// would leak into child processes.
func TestMain(m *testing.M) {
	for _, k := range []string{
		"WARDCASE_PROXY", "WARDCASE_GATEWAY", "WARDCASE_RUN", "ANTHROPIC_BASE_URL",
		"WARDCASE_MCP", "WARDCASE_MCP_SERVERS", "WARDCASE_MARKETPLACES",
		"WARDCASE_PLUGINS", "WARDCASE_SKILL_PATHS",
	} {
		_ = os.Unsetenv(k)
	}
	os.Exit(m.Run())
}
