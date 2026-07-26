package main

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

	"tools.xdoubleu.com/internal/config"
)

// writeStubScript writes an executable shell script standing in for `node`
// and returns its path. WebNodeBin/WebServerJS only ever combine into
// exec.Command(webNodeBin, webServerJS), so a shebang script that ignores
// its single argument is a faithful stand-in.
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

func testWebConfig(t *testing.T, script string) config.Config {
	t.Helper()
	//nolint:exhaustruct //only the fields startWebProcess/webProcessEnv read are needed
	return config.Config{
		WebNodeBin:      script,
		WebServerJS:     "unused",
		WebPort:         3000,
		APIURL:          "http://localhost:8000",
		SupabaseURL:     "http://supabase.local",
		SupabaseAnonKey: "anon-key",
		Release:         "test-release",
	}
}

func TestStartWebProcess_StartAndStop(t *testing.T) {
	script := writeStubScript(t, "while :; do :; done\n")
	cfg := testWebConfig(t, script)

	w, err := startWebProcess(context.Background(), logging.NewNopLogger(), cfg)
	require.NoError(t, err)

	require.NotNil(t, w.cmd.Process)
	w.Stop()

	select {
	case <-w.done:
	case <-time.After(webProcessStopTimeout + time.Second):
		t.Fatal("web process did not exit after Stop")
	}
}

func TestStartWebProcess_UnexpectedExitTriggersShutdown(t *testing.T) {
	orig := shutdownSignal
	var called atomic.Bool
	shutdownSignal = func() { called.Store(true) }
	t.Cleanup(func() { shutdownSignal = orig })

	script := writeStubScript(t, "exit 0\n")
	cfg := testWebConfig(t, script)

	w, err := startWebProcess(context.Background(), logging.NewNopLogger(), cfg)
	require.NoError(t, err)

	select {
	case <-w.done:
	case <-time.After(5 * time.Second):
		t.Fatal("web process did not exit")
	}
	// waitAndSignalOnUnexpectedExit closes done then checks stopping before
	// calling shutdownSignal; give that goroutine a moment to run.
	assert.Eventually(t, called.Load, time.Second, 10*time.Millisecond)
}

func TestStartWebProcess_StopSuppressesShutdownSignal(t *testing.T) {
	orig := shutdownSignal
	var called atomic.Bool
	shutdownSignal = func() { called.Store(true) }
	t.Cleanup(func() { shutdownSignal = orig })

	script := writeStubScript(t, "while :; do :; done\n")
	cfg := testWebConfig(t, script)

	w, err := startWebProcess(context.Background(), logging.NewNopLogger(), cfg)
	require.NoError(t, err)

	w.Stop()

	assert.Never(t, called.Load, 200*time.Millisecond, 10*time.Millisecond)
}

func TestStartWebProcess_StartFailureReturnsError(t *testing.T) {
	cfg := testWebConfig(t, filepath.Join(t.TempDir(), "does-not-exist"))

	_, err := startWebProcess(context.Background(), logging.NewNopLogger(), cfg)

	require.Error(t, err)
}

func TestStop_EscalatesToKillWhenChildIgnoresSigterm(t *testing.T) {
	origTimeout := webProcessStopTimeout
	webProcessStopTimeout = 100 * time.Millisecond
	t.Cleanup(func() { webProcessStopTimeout = origTimeout })

	script := writeStubScript(t, "trap '' TERM\nwhile :; do :; done\n")
	cfg := testWebConfig(t, script)

	w, err := startWebProcess(context.Background(), logging.NewNopLogger(), cfg)
	require.NoError(t, err)
	// Give the child shell time to actually execute the `trap` line before
	// Stop sends SIGTERM — otherwise SIGTERM can race ahead of trap
	// registration and kill the child immediately (default disposition),
	// masking the escalation path this test exists to cover.
	time.Sleep(200 * time.Millisecond)

	done := make(chan struct{})
	go func() {
		w.Stop()
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
	w := &webProcess{cmd: &exec.Cmd{}, done: make(chan struct{})}

	require.NotPanics(t, w.Stop)
}

func TestWebProcessEnv(t *testing.T) {
	cfg := testWebConfig(t, "node")

	env := webProcessEnv(cfg)

	assert.Contains(t, env, "PORT=3000")
	assert.Contains(t, env, "RELEASE=test-release")
	assert.Contains(t, env, "API_URL=http://localhost:8000")
	assert.Contains(t, env, "SUPABASE_URL=http://supabase.local")
	assert.Contains(t, env, "SUPABASE_ANON_KEY=anon-key")
	for _, e := range env {
		assert.NotContains(t, e, "PORT=8000", "parent PORT must not leak to the child")
	}
}
