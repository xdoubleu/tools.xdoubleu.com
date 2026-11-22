package ward_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/wardcase/internal/ward"
)

// repoFixture is a signed-in "fake" server and a worktree holding repoFile (when
// not empty); the session's git top level is that worktree.
func repoFixture(t *testing.T, repoFile string) *fixture {
	t.Helper()
	f := newFakeMCP(t)
	fx, store := mcpFixture(t, f, false)
	login(t, f, store)
	wt := t.TempDir()
	if repoFile != "" {
		require.NoError(t, os.MkdirAll(filepath.Join(wt, ".wardcase"), 0o750))
		require.NoError(t, os.WriteFile(
			filepath.Join(wt, ".wardcase", "config.toml"), []byte(repoFile), 0o600))
	}
	fx.r.Git = fakeGit(wt, wt+"/.git")
	return fx
}

func TestNoRepoFileGivesEveryConfiguredServer(t *testing.T) {
	fx := repoFixture(t, "")
	cfg, _ := proxyConfig(t, fx, false)
	assert.Len(t, cfg.MCPServers, 1)
}

func TestAnEmptyMCPListGivesNoServers(t *testing.T) {
	fx := repoFixture(t, "mcp = []\n")
	cfg, env := proxyConfig(t, fx, false)
	assert.Empty(t, cfg.MCPServers)
	assert.NotContains(t, env, "WARDCASE_MCP_SERVERS")
}

func TestARepoListNarrowsToTheNamesTheDeviceHas(t *testing.T) {
	fx := repoFixture(t, "mcp = [\"fake\"]\nplugins = [\"m/p\"]\n")
	cfg, _ := proxyConfig(t, fx, false)
	assert.Len(t, cfg.MCPServers, 1)
}

func TestARepoNameTheDeviceLacksIsAWarning(t *testing.T) {
	fx := repoFixture(t, "mcp = [\"fake\", \"posthog\"]\n")
	cfg, _ := proxyConfig(t, fx, false)
	assert.Len(t, cfg.MCPServers, 1)
	assert.Contains(t, fx.notes.String(), "repo lists MCP server posthog")
}

func TestTheRepoNarrowsJobsAfterTheJobsFilter(t *testing.T) {
	fx := repoFixture(t, "mcp = [\"fake\"]\n")
	cfg, _ := proxyConfig(t, fx, true)
	assert.Empty(t, cfg.MCPServers, "jobs=false still wins")
}

func TestARepoFileCannotDefineAServer(t *testing.T) {
	for _, body := range []string{
		"url = \"https://evil.example/mcp\"\n",
		"[[mcp]]\nname = \"x\"\nurl = \"https://evil.example\"\n",
		"[headers]\nAuthorization = \"x\"\n",
	} {
		_, err := ward.LoadRepoConfig(writeRepo(t, body))
		require.Error(t, err, body)
	}
	_, err := ward.LoadRepoConfig(writeRepo(t, "url = \"https://e\"\n"))
	assert.ErrorContains(t, err, "servers belong in the device config")
}

func TestARepoPluginMustBeMarketplaceSlashPlugin(t *testing.T) {
	_, err := ward.LoadRepoConfig(writeRepo(t, "plugins = [\"bare\"]\n"))
	assert.ErrorContains(t, err, "marketplace/plugin")
}

func TestTheRepoFileIsReadFromTheSessionWorktreeNotTheCallersCheckout(t *testing.T) {
	fx := repoFixture(t, "")
	caller := writeRepo(t, "mcp = []\n")
	_, err := ward.LoadRepoConfig(caller)
	require.NoError(t, err)
	fx.r.Config.MCP[0].Name = "fake"
	cfg, _ := proxyConfig(t, fx, false) // Request.Dir "/w"; top level is the worktree
	assert.Len(t, cfg.MCPServers, 1, "the caller's mcp = [] is not applied")
}

func writeRepo(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".wardcase"), 0o750))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, ".wardcase", "config.toml"), []byte(body), 0o600))
	return dir
}
