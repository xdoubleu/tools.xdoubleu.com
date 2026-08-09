//nolint:testpackage //mutates unexported childProcessStopTimeout var
package gateway

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/logging"
)

// writeStubScript writes an executable shell script and returns its path.
func writeStubScript(t *testing.T, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("stub relies on a POSIX shebang script")
	}

	path := filepath.Join(t.TempDir(), "stub.sh")
	script := "#!/bin/sh\n" + body
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))

	return path
}

func stubCmd(t *testing.T, body string) *exec.Cmd {
	t.Helper()
	script := writeStubScript(t, body)
	cmd := exec.CommandContext(context.Background(), script)
	cmd.Stdout = nil
	cmd.Stderr = nil

	return cmd
}

func TestStartChildProcess_StartAndStop(t *testing.T) {
	c, err := StartChildProcess(
		logging.NewNopLogger(), "stub", stubCmd(t, "while :; do :; done\n"),
	)
	require.NoError(t, err)

	require.NotNil(t, c.cmd.Process)
	c.Stop()

	select {
	case <-c.done:
	case <-time.After(childProcessStopTimeout + time.Second):
		t.Fatal("child process did not exit after Stop")
	}
}

func TestStartChildProcess_UnexpectedExitTriggersShutdown(t *testing.T) {
	orig := shutdownSignal
	var called atomic.Bool
	shutdownSignal = func() { called.Store(true) }
	t.Cleanup(func() { shutdownSignal = orig })

	c, err := StartChildProcess(logging.NewNopLogger(), "stub", stubCmd(t, "exit 0\n"))
	require.NoError(t, err)

	select {
	case <-c.done:
	case <-time.After(5 * time.Second):
		t.Fatal("child process did not exit")
	}
	assert.Eventually(t, called.Load, time.Second, 10*time.Millisecond)
}

func TestStartChildProcess_StopSuppressesShutdownSignal(t *testing.T) {
	orig := shutdownSignal
	var called atomic.Bool
	shutdownSignal = func() { called.Store(true) }
	t.Cleanup(func() { shutdownSignal = orig })

	c, err := StartChildProcess(
		logging.NewNopLogger(), "stub", stubCmd(t, "while :; do :; done\n"),
	)
	require.NoError(t, err)

	c.Stop()

	assert.Never(t, called.Load, 200*time.Millisecond, 10*time.Millisecond)
}

func TestStartChildProcess_StartFailureReturnsError(t *testing.T) {
	cmd := exec.CommandContext(
		context.Background(), filepath.Join(t.TempDir(), "does-not-exist"),
	)

	_, err := StartChildProcess(logging.NewNopLogger(), "stub", cmd)

	require.Error(t, err)
}

func TestStop_EscalatesToKillWhenChildIgnoresSigterm(t *testing.T) {
	origTimeout := childProcessStopTimeout
	childProcessStopTimeout = 100 * time.Millisecond
	t.Cleanup(func() { childProcessStopTimeout = origTimeout })

	c, err := StartChildProcess(
		logging.NewNopLogger(), "stub",
		stubCmd(t, "trap '' TERM\nwhile :; do :; done\n"),
	)
	require.NoError(t, err)
	// Give the child shell time to actually execute the `trap` line before
	// Stop sends SIGTERM — otherwise SIGTERM can race ahead of trap
	// registration and kill the child immediately (default disposition),
	// masking the escalation path this test exists to cover.
	time.Sleep(200 * time.Millisecond)

	done := make(chan struct{})
	go func() {
		c.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop did not escalate to SIGKILL in time")
	}
}

func TestStop_NilProcessIsANoop(t *testing.T) {
	//nolint:exhaustruct //zero-value cmd (nil Process) is the point of this test
	c := &ChildProcess{cmd: &exec.Cmd{}, done: make(chan struct{})}

	require.NotPanics(t, c.Stop)
}
