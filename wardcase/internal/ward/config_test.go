package ward_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/wardcase/internal/ward"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}

func TestAMissingConfigIsClaudeThenOpenCodeAskingFirst(t *testing.T) {
	cfg, err := ward.LoadConfig(filepath.Join(t.TempDir(), "none.toml"))
	require.NoError(t, err)
	assert.Equal(t, []string{"claude", "opencode"}, cfg.Order)
	assert.Equal(t, "ask", cfg.Overflow)
}

func TestTheConfigIsReadAsWritten(t *testing.T) {
	cfg, err := ward.LoadConfig(writeConfig(t, `order = ["opencode", "claude"]
overflow = "auto"
image = "factory-job:abc"

[opencode]
model = "vendor/model:free"
`))
	require.NoError(t, err)
	assert.Equal(t, []string{"opencode", "claude"}, cfg.Order)
	assert.Equal(t, "auto", cfg.Overflow)
	assert.Equal(t, "factory-job:abc", cfg.Image)
	assert.Equal(t, "vendor/model:free", cfg.OpenCode.Model)
}

func TestABadConfigIsRefused(t *testing.T) {
	for name, body := range map[string]string{
		"unknown harness":  `order = ["hermes"]`,
		"twice":            `order = ["claude", "claude"]`,
		"overflow":         `overflow = "maybe"`,
		"model":            "[opencode]\nmodel = \"no slash\"",
		"unknown setting":  `colour = "blue"`,
		"not toml":         `order = [`,
		"unreadable entry": "[opencode]\nmodel = 3",
	} {
		_, err := ward.LoadConfig(writeConfig(t, body))
		assert.Error(t, err, name)
	}
}

func TestAnUnreadableConfigIsAnError(t *testing.T) {
	_, err := ward.LoadConfig(t.TempDir())
	assert.Error(t, err)
}

func TestTheConfigLivesUnderDotConfig(t *testing.T) {
	assert.Equal(t, "/h/.config/wardcase/config.toml", ward.ConfigPath("/h"))
}

func TestCodexCanBeInTheOrder(t *testing.T) {
	cfg, err := ward.LoadConfig(writeConfig(t, `order = ["claude", "opencode", "codex"]`))
	require.NoError(t, err)
	assert.Equal(t, []string{"claude", "opencode", "codex"}, cfg.Order)
}
