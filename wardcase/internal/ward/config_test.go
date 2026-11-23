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

[opencode]
model = "vendor/model:free"
`))
	require.NoError(t, err)
	assert.Equal(t, []string{"opencode", "claude"}, cfg.Order)
	assert.Equal(t, "auto", cfg.Overflow)
	assert.Equal(t, "vendor/model:free", cfg.OpenCode.Model)
}

func TestTheDeviceConfigCannotPickAnImage(t *testing.T) {
	_, err := ward.LoadConfig(writeConfig(t, `image = "evil:latest"`))
	require.ErrorContains(t, err, "unknown setting image")
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

func TestMCPServersAreReadWithJobsOffByDefault(t *testing.T) {
	cfg, err := ward.LoadConfig(writeConfig(t, `[[mcp]]
name = "posthog"
url = "https://mcp.posthog.com/mcp"

[[mcp]]
name = "local"
url = "http://localhost:8080/mcp"
jobs = true
`))
	require.NoError(t, err)
	require.Len(t, cfg.MCP, 2)
	assert.False(t, cfg.MCP[0].Jobs)
	assert.True(t, cfg.MCP[1].Jobs)
	got, ok := cfg.MCPServer("local")
	assert.True(t, ok)
	assert.Equal(t, "http://localhost:8080/mcp", got.URL)
	_, ok = cfg.MCPServer("nope")
	assert.False(t, ok)
}

func TestABadMCPEntryIsRefused(t *testing.T) {
	cases := map[string]string{
		"unknown key": "[[mcp]]\nname = \"a\"\nurl = \"https://x.example\"\ntoken = \"t\"\n",
		"duplicate": "[[mcp]]\nname = \"a\"\nurl = \"https://x.example\"\n" +
			"[[mcp]]\nname = \"a\"\nurl = \"https://y.example\"\n",
		"reserved name":  "[[mcp]]\nname = \"tools-apps\"\nurl = \"https://x.example\"\n",
		"bad name":       "[[mcp]]\nname = \"A b\"\nurl = \"https://x.example\"\n",
		"plain http":     "[[mcp]]\nname = \"a\"\nurl = \"http://x.example/mcp\"\n",
		"no url":         "[[mcp]]\nname = \"a\"\n",
		"url credential": "[[mcp]]\nname = \"a\"\nurl = \"https://u:p@x.example\"\n",
	}
	for name, body := range cases {
		_, err := ward.LoadConfig(writeConfig(t, body))
		assert.Error(t, err, name)
	}
}
