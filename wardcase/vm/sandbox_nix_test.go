package vm_test

import (
	"context"
	"io"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/wardcase/vm"
)

const stubNix = `#!/bin/sh
echo "$*" >> "$CALLS"
[ "$1" != store ] || echo "gc: cannot access gc-socket" >&2
[ "$1" = print-dev-env ] || exit 0
[ -z "$FAIL_DEVENV" ] || { echo "error: flake broke" >&2; exit 1; }
echo "export DEVENV=stub https_proxy=leaked"
`

// stubDF reports USED_KB as the volume's used space, in df -Pk's layout.
const stubDF = `#!/bin/sh
echo "Filesystem 1024-blocks Used Available Capacity Mounted on"
echo "store 100000000 ${USED_KB:-0} 0 0% $2"
`

const lockFile = `{"nodes":{
"nixpkgs":{"locked":{"type":"github","owner":"NixOS","repo":"nixpkgs",
"rev":"abc123"}},
"local":{"locked":{"type":"path","path":"./x"}},
"root":{"inputs":{"nixpkgs":"nixpkgs","local":"local","follow":["a","b"]}}},
"root":"root"}`

func write(t *testing.T, path, body string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
}

// flakeCheckout is a checkout with a flake.nix, and a flake.lock when lock is set.
func flakeCheckout(t *testing.T, lock string) string {
	t.Helper()
	dir := t.TempDir()
	write(t, filepath.Join(dir, "flake.nix"), "{}")
	if lock != "" {
		write(t, filepath.Join(dir, "flake.lock"), lock)
	}
	return dir
}

// nixSession writes a Nix root whose profile has the stub nix, a checkout, and
// runs the nix setup script followed by a command; it returns what nix was called
// with.
func nixSession(
	t *testing.T, root, checkout, proxyURL string, box *vm.ContainerSandbox,
	env ...string,
) (string, string) {
	t.Helper()
	bin := filepath.Join(root, "var", "nix", "profiles", "default", "bin")
	require.NoError(t, os.MkdirAll(bin, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "store"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bin, "nix"), []byte(stubNix), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bin, "df"), []byte(stubDF), 0o755))
	callLog := filepath.Join(t.TempDir(), "calls")
	script := vm.NixScriptForTest(box, root, t.TempDir(), proxyURL) + `exec "$@"`
	cmd := osexec.Command("sh", "-c", script, "sh",
		"sh", "-c", "echo ran in $(basename $PWD) devenv=$DEVENV proxy=$https_proxy")
	cmd.Dir = checkout
	cmd.Env = append(os.Environ(), append([]string{"CALLS=" + callLog}, env...)...)
	got, err := cmd.CombinedOutput()
	require.NoError(t, err, string(got))
	data, _ := os.ReadFile(callLog)
	return string(data), string(got)
}

func nixBox(t *testing.T, limit int64) *vm.ContainerSandbox {
	box := sandboxWith(t, newExecScript(routesAnswer))
	box.NixStore, box.NixStoreMax = t.TempDir(), limit
	return box
}

func TestACheckoutWithoutAFlakeRunsItsCommandDirectly(t *testing.T) {
	calls, out := nixSession(t, t.TempDir(), t.TempDir(), "http://gw:8118", nixBox(t, 0))

	assert.Empty(t, calls, "no nix develop, no gc")
	assert.Contains(t, out, "ran in")
}

func TestACheckoutWithAFlakeRunsInsideItsCachedDevShell(t *testing.T) {
	checkout := flakeCheckout(t, lockFile)
	root := t.TempDir()

	calls, out := nixSession(t, root, checkout, "http://gw:8118", nixBox(t, 0))

	assert.Contains(t, out, "ran in "+filepath.Base(checkout)+" devenv=stub proxy=\n")
	profile := filepath.Join(root, "var", "nix", "profiles",
		"devshell-"+filepath.Base(checkout))
	assert.Equal(t, "profile wipe-history --profile "+profile+"\n"+
		"print-dev-env --profile "+profile+" --no-write-lock-file --override-input "+
		"nixpkgs tarball+http://gw:8118/nixsrc/NixOS/nixpkgs/tar.gz/abc123\n", calls)
	envs, err := filepath.Glob(filepath.Join(root, "var", "wardcase", "devenv", "*.sh"))
	require.NoError(t, err)
	assert.Len(t, envs, 1)
}

func TestAnUnchangedFlakeReusesItsDevShellWithoutEvaluating(t *testing.T) {
	checkout := flakeCheckout(t, lockFile)
	root := t.TempDir()
	nixSession(t, root, checkout, "http://gw:8118", nixBox(t, 0))
	profile := filepath.Join(root, "var", "nix", "profiles",
		"devshell-"+filepath.Base(checkout))
	require.NoError(t, os.WriteFile(profile, nil, 0o600))

	warm, out := nixSession(t, root, checkout, "http://gw:8118", nixBox(t, 0))
	write(t, filepath.Join(checkout, "flake.lock"), lockFile+" ")
	changed, _ := nixSession(t, root, checkout, "http://gw:8118", nixBox(t, 0))

	assert.Empty(t, warm, "a cached dev shell calls no nix")
	assert.Contains(t, out, "devenv=stub proxy=\n")
	assert.Contains(t, changed, "print-dev-env")
	envs, err := filepath.Glob(filepath.Join(root, "var", "wardcase", "devenv", "*.sh"))
	require.NoError(t, err)
	assert.Len(t, envs, 1, "a new hash replaces the repo's old environment")
}

func TestADevShellThatFailsToBuildFailsTheSessionAndCachesNothing(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "var", "nix", "profiles", "default", "bin")
	require.NoError(t, os.MkdirAll(bin, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "store"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bin, "nix"), []byte(stubNix), 0o755))
	script := vm.NixScriptForTest(nixBox(t, 0), root, t.TempDir(), "") + `exec "$@"`
	cmd := osexec.Command("sh", "-c", script, "sh", "true")
	cmd.Dir = flakeCheckout(t, "")
	cmd.Env = append(os.Environ(), "CALLS=/dev/null", "FAIL_DEVENV=1")

	out, err := cmd.CombinedOutput()

	require.Error(t, err)
	assert.Contains(t, string(out), "error: flake broke")
	assert.Contains(t, string(out), "wardcase: building the flake dev shell failed")
	envs, _ := filepath.Glob(filepath.Join(root, "var", "wardcase", "devenv", "*"))
	assert.Empty(t, envs)
	assert.NoDirExists(t, filepath.Join(root, "var", "wardcase.lock"))
}

func TestAFlakeWithoutAProxyIsEnteredWithoutOverrides(t *testing.T) {
	checkout := flakeCheckout(t, "")

	calls, _ := nixSession(t, t.TempDir(), checkout, "", nixBox(t, 0))

	assert.Contains(t, calls, "print-dev-env --profile ")
	assert.NotContains(t, calls, "--override-input")
}

func TestTheStoreIsCollectedOnlyPastItsCap(t *testing.T) {
	root := t.TempDir()

	under, _ := nixSession(t, root, t.TempDir(), "", nixBox(t, 1<<30), "USED_KB=1000")
	over, _ := nixSession(t, root, t.TempDir(), "", nixBox(t, 1<<20), "USED_KB=1025")

	assert.Empty(t, under, "a second session under the cap reuses the store as it is")
	assert.Equal(t, "store gc\n", over)
}

func TestTheStoresCollectionLogsToAFileNotTheJobsOutput(t *testing.T) {
	root := t.TempDir()

	calls, out := nixSession(t, root, t.TempDir(), "", nixBox(t, 1024), "USED_KB=2")

	assert.Equal(t, "store gc\n", calls)
	assert.NotContains(t, out, "gc-socket")
	logged, err := os.ReadFile(filepath.Join(root, "var", "log", "wardcase-gc.log"))
	require.NoError(t, err)
	assert.Contains(t, string(logged), "gc: cannot access gc-socket")
}

func TestTheFirstSessionSeedsAnEmptyStoreAndLaterOnesKeepIt(t *testing.T) {
	root, seed := t.TempDir(), t.TempDir()
	bin := filepath.Join(seed, "var", "nix", "profiles", "default", "bin")
	require.NoError(t, os.MkdirAll(bin, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bin, "nix"), []byte(stubNix), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "store"), 0o755))
	checkout := flakeCheckout(t, "")
	callLog := filepath.Join(t.TempDir(), "calls")
	script := vm.NixScriptForTest(nixBox(t, 0), root, seed, "") + `exec "$@"`
	for range 2 {
		cmd := osexec.Command("sh", "-c", script, "sh", "true")
		cmd.Dir, cmd.Env = checkout, append(os.Environ(), "CALLS="+callLog)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
		require.NoError(t, os.WriteFile(filepath.Join(root, "store", "kept"), nil, 0o600))
	}

	assert.FileExists(t,
		filepath.Join(root, "var", "nix", "profiles", "default", "bin", "nix"))
	assert.FileExists(t, filepath.Join(root, "store", "kept"))
}

func TestWithoutAnyNixStoreTheSandboxIgnoresFlakes(t *testing.T) {
	exec := newExecScript(routesAnswer)
	_, err := sandboxWith(t, exec).Run(context.Background(), spec(t, io.Discard))
	require.NoError(t, err)

	assert.NotContains(t, strings.Join(exec.find("run")[0], " "), "target=/nix")
	assert.NotContains(t, strings.Join(exec.find("exec")[0], " "), "nix develop")
}

func TestTheNixStoreIsMountedReadWriteAndFlakesAreEntered(t *testing.T) {
	exec := newExecScript(routesAnswer)
	box := nixBox(t, 0)
	box.Exec = exec

	_, err := box.Run(context.Background(), spec(t, io.Discard))
	require.NoError(t, err)

	run := strings.Join(exec.find("run")[0], " ")
	assert.Contains(t, run, "source="+box.NixStore+",target=/nix")
	assert.NotContains(t, run, "target=/nix,readonly")
	execLine := strings.Join(exec.find("exec")[0], " ")
	assert.Contains(t, execLine, "nix print-dev-env")
	assert.Contains(t, execLine, "df -Pk /nix")
	assert.NotContains(t, execLine, "du -sk")
	assert.Contains(t, execLine, "-le 16777216")
}

func TestASeedThatCannotBeCopiedFailsTheSessionLoudly(t *testing.T) {
	script := vm.NixScriptForTest(nixBox(t, 0), t.TempDir(),
		filepath.Join(t.TempDir(), "missing"), "") + `exec "$@"`

	out, err := osexec.Command("sh", "-c", script, "sh", "true").CombinedOutput()

	require.Error(t, err)
	assert.Contains(t, string(out), "wardcase: seeding")
}

func TestAPopulatedStoreWithoutItsProfileIsNotSeededOver(t *testing.T) {
	root, seed := t.TempDir(), t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "store", "abc-hello"), 0o755))
	write(t, filepath.Join(seed, "seeded"), "x")
	script := vm.NixScriptForTest(nixBox(t, 0), root, seed, "") + `exec "$@"`

	out, err := osexec.Command("sh", "-c", script, "sh", "true").CombinedOutput()

	require.Error(t, err)
	assert.Contains(t, string(out), "wardcase: /nix is not empty")
	assert.NoFileExists(t, filepath.Join(root, "seeded"))
	assert.NoDirExists(t, filepath.Join(root, "var", "wardcase.lock"),
		"the lock is released")
}

func TestASessionWaitsForTheStoreLockAndReleasesIt(t *testing.T) {
	root := t.TempDir()
	lock := filepath.Join(root, "var", "wardcase.lock")
	require.NoError(t, os.MkdirAll(lock, 0o755))
	go func() {
		time.Sleep(1500 * time.Millisecond)
		_ = os.Remove(lock)
	}()
	began := time.Now()

	_, out := nixSession(t, root, t.TempDir(), "", nixBox(t, 0))

	assert.Contains(t, out, "ran in")
	assert.GreaterOrEqual(t, time.Since(began), time.Second)
	assert.NoDirExists(t, lock)
}

func TestAStaleStoreLockIsBroken(t *testing.T) {
	root := t.TempDir()
	lock := filepath.Join(root, "var", "wardcase.lock")
	require.NoError(t, os.MkdirAll(lock, 0o755))
	old := time.Now().Add(-time.Hour)
	require.NoError(t, os.Chtimes(lock, old, old))

	_, out := nixSession(t, root, t.TempDir(), "", nixBox(t, 0))

	assert.Contains(t, out, "ran in")
	assert.NoDirExists(t, lock)
}
