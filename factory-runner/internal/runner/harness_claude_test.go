package runner_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

func writeFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), mode))
}

// runClaude runs the harness's argv with a fake `claude` that records its arguments,
// against a seed directory holding one file and the given session directory.
func runClaude(t *testing.T, argv []string, seed, session string) string {
	t.Helper()
	bin := t.TempDir()
	out := filepath.Join(bin, "args.txt")
	script := "#!/bin/sh\necho \"$@\" > '" + out + "'\n"
	writeFile(t, filepath.Join(bin, "claude"), script, 0o755)

	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Env = append(os.Environ(),
		"PATH="+bin+":"+os.Getenv("PATH"),
		"CLAUDE_CONFIG_DIR="+session,
		"CLAUDE_CONFIG_SEED="+seed,
	)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
	got, err := os.ReadFile(out)
	require.NoError(t, err)
	return string(got)
}

func TestANewSessionDirectoryGetsTheImagesClaudeConfigOnce(t *testing.T) {
	seed, session := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(seed, "settings.json"), "a", 0o600)

	got := runClaude(t, runner.ClaudeCode{}.Command(), seed, session)
	assert.Contains(t, got, "-p --output-format stream-json")
	assert.FileExists(t, filepath.Join(session, "settings.json"))

	// A later job changes the config, then resumes: the directory is left alone.
	writeFile(t, filepath.Join(session, "settings.json"), "b", 0o600)
	writeFile(t, filepath.Join(seed, "settings.json"), "c", 0o600)
	got = runClaude(t, runner.ClaudeCode{}.Resume("abc"), seed, session)
	assert.Contains(t, got, "--resume abc")
	content, err := os.ReadFile(filepath.Join(session, "settings.json"))
	require.NoError(t, err)
	assert.Equal(t, "b", string(content))
}

func TestAMissingConfigSeedStopsTheJobBeforeClaudeRuns(t *testing.T) {
	bin := t.TempDir()
	writeFile(t, filepath.Join(bin, "claude"), "#!/bin/sh\n", 0o755)
	argv := runner.ClaudeCode{}.Command()
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Env = append(os.Environ(),
		"PATH="+bin+":"+os.Getenv("PATH"),
		"CLAUDE_CONFIG_DIR="+t.TempDir(),
		"CLAUDE_CONFIG_SEED="+filepath.Join(t.TempDir(), "missing"),
	)
	require.Error(t, cmd.Run())
}

func TestClaudeCodeSessionEnvNamesTheConfigAndItsSeed(t *testing.T) {
	env := runner.ClaudeCode{}.SessionEnv("/session")
	assert.Equal(t, "/session", env["CLAUDE_CONFIG_DIR"])
	assert.Equal(t, runner.ClaudeConfigSeed, env["CLAUDE_CONFIG_SEED"])
}
