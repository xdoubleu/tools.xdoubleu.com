//go:build container

package vm_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/wardcase/vm"
)

// Runs real VMs: `go test -tags container -run TestRealVM ./vm` on a Mac
// with `container` and the image from `make image`. CI never runs it.
func TestRealVMRunsAJobIsolatedAndCleansUp(t *testing.T) {
	box := &vm.ContainerSandbox{
		Image:      "factory-job:dev",
		Exec:       vm.OSExecer{BinDirs: nil},
		Proxy:      nil,
		GoModCache: "",
		NPMCache:   "",
		CPUs:       "",
		Memory:     "",
		TempDir:    t.TempDir(),
	}
	require.NoError(t, box.Check(context.Background()))

	dir := t.TempDir()
	clone, outcome, session := filepath.Join(dir, "clone"), filepath.Join(dir, "out"),
		filepath.Join(dir, "session")
	for _, d := range []string{clone, outcome, session} {
		require.NoError(t, os.MkdirAll(d, 0o777))
		require.NoError(t, os.Chmod(d, 0o777))
	}
	script := `
set -u
echo "uid=$(id -u) home=$HOME secret=$SECRET"
echo "cwd=$(pwd)"
curl -sS -m 4 -o /dev/null https://example.com && echo "NET=leak" || echo "NET=blocked"
touch /x 2>/dev/null && echo "ROOT=writable" || echo "ROOT=readonly"
ls /Users 2>/dev/null | head -n 1 | grep -q . && echo "HOST=visible" || echo "HOST=hidden"
echo done > /work/file && echo done > /outcome/outcome.md && echo done > /session/s
git -C /work init -q && git -C /work status -s >/dev/null && echo "GIT=ok"
echo "STDIN=$(cat)"
`
	var log bytes.Buffer
	code, err := box.Run(context.Background(), vm.SandboxSpec{
		CloneDir:   clone,
		OutcomeDir: outcome,
		SessionDir: session,
		Env:        map[string]string{"WARDCASE_RUN": "realvm-test", "SECRET": "s3cret"},
		Command:    []string{"sh", "-c", script},
		Stdin:      "prompt text",
		Log:        &log,
	})
	require.NoError(t, err, log.String())
	out := log.String()
	assert.Zero(t, code, out)
	for _, want := range []string{
		"uid=1000 home=/tmp/home secret=s3cret", "cwd=/work", "NET=blocked",
		"ROOT=readonly", "HOST=hidden", "GIT=ok", "STDIN=prompt text",
	} {
		assert.Contains(t, out, want)
	}
	for _, f := range []string{
		filepath.Join(clone, "file"), filepath.Join(outcome, "outcome.md"),
		filepath.Join(session, "s"),
	} {
		assert.FileExists(t, f)
	}

	var left bytes.Buffer
	_, err = vm.OSExecer{BinDirs: nil}.Exec(context.Background(),
		[]string{"container", "ls", "-a", "--quiet"}, nil, &left)
	require.NoError(t, err)
	assert.NotContains(t, left.String(), "wardcase-realvm")
}

func TestRealVMSeedsClaudesConfigSoThePluginsAreVisible(t *testing.T) {
	box := &vm.ContainerSandbox{
		Image:      "factory-job:dev",
		Exec:       vm.OSExecer{BinDirs: nil},
		Proxy:      nil,
		GoModCache: "",
		NPMCache:   "",
		CPUs:       "",
		Memory:     "",
		TempDir:    t.TempDir(),
	}
	dir := t.TempDir()
	clone, outcome, session := filepath.Join(dir, "clone"), filepath.Join(dir, "out"),
		filepath.Join(dir, "session")
	for _, d := range []string{clone, outcome, session} {
		require.NoError(t, os.MkdirAll(d, 0o777))
		require.NoError(t, os.Chmod(d, 0o777))
	}
	// The harness's own start step, with `claude plugin list` in place of the agent.
	command := append(vm.ClaudeCode{}.Command()[:4], "claude", "plugin", "list")
	var log bytes.Buffer
	code, err := box.Run(context.Background(), vm.SandboxSpec{
		CloneDir:   clone,
		OutcomeDir: outcome,
		SessionDir: session,
		Env: map[string]string{
			"WARDCASE_RUN":       "realvm-seed",
			"CLAUDE_CONFIG_DIR":  vm.SandboxSessionDir,
			"CLAUDE_CONFIG_SEED": vm.ClaudeConfigSeed,
		},
		Command: command,
		Stdin:   "",
		Log:     &log,
	})
	require.NoError(t, err, log.String())
	assert.Zero(t, code, log.String())
	for _, plugin := range []string{
		"git-task-flow", "session-retro", "mattpocock-skills", "claude-md-management",
	} {
		assert.Contains(t, log.String(), plugin)
	}
	assert.FileExists(t, filepath.Join(session, ".seeded"))
	assert.True(t, strings.Contains(log.String(), "xdoubleu-skills"), log.String())
}
