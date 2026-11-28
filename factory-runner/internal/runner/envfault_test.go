package runner_test

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

// deadNix is a sandbox run whose Nix mount died: the harness never starts.
func deadNix(_ context.Context, spec runner.SandboxSpec) (int, error) {
	_, _ = io.WriteString(spec.Log, "env: 'nix': No such device or address\n")
	return 127, nil
}

func TestABrokenNixMountFailsOneJobAndHoldsTheQueueUntilPreflightPasses(t *testing.T) {
	second := newJob()
	second.ID = "22222222-2222-4222-8222-222222222222"
	env := newEnv(t, deadNix, newJob(), second)
	now := time.Now()
	env.runner.Now = func() time.Time { return now }
	var healthy atomic.Bool
	var preflights atomic.Int32
	env.runner.Preflight = func(context.Context) error {
		preflights.Add(1)
		if healthy.Load() {
			return nil
		}
		return errors.New("Device not configured")
	}
	// The mount dies after the preflight, so the first job meets it.
	healthy.Store(true)
	ran, err := env.runner.Next(context.Background())
	require.NoError(t, err)
	assert.True(t, ran)
	healthy.Store(false)

	done := env.api.done()
	require.Len(t, done, 1)
	assert.False(t, done[0].succeeded)
	assert.True(t, done[0].limited, "the task is queued again, not failed")
	assert.Equal(t, "Runner environment: the Nix store is unreadable", done[0].blocked)
	assert.Contains(t, done[0].outcome, "goes back to the queue untouched")
	assert.Contains(t, env.runner.Fault(), "the Nix store is unreadable")

	// Held: the second job stays queued, however often the loop asks.
	for range 3 {
		ran, err = env.runner.Next(context.Background())
		require.NoError(t, err)
		assert.False(t, ran)
		now = now.Add(time.Minute)
	}
	assert.Len(t, env.api.done(), 1)
	assert.NotEmpty(t, env.runner.Fault())

	// The mount comes back; the next probe closes the breaker and claims resume.
	healthy.Store(true)
	env.sandbox.run = outcomeOnly("Did it.\nChecks: lint=pass")
	now = now.Add(time.Minute)
	ran, err = env.runner.Next(context.Background())
	require.NoError(t, err)
	assert.True(t, ran)
	assert.Empty(t, env.runner.Fault())
	require.Len(t, env.api.done(), 2)
	assert.Equal(t, second.ID, env.api.done()[1].jobID)
}

func TestAFailingPreflightClaimsNothing(t *testing.T) {
	env := newEnv(t, outcomeOnly("x"), newJob())
	env.runner.Preflight = func(context.Context) error { return errors.New("dead") }

	ran, err := env.runner.Next(context.Background())
	require.NoError(t, err)
	assert.False(t, ran)
	assert.Empty(t, env.api.done())
	assert.Contains(t, env.runner.Fault(), "preflight failed: dead")
}

func TestAnAgentFailureAfterItsSessionStartedIsNotTheEnvironments(t *testing.T) {
	env := newEnv(t, func(_ context.Context, spec runner.SandboxSpec) (int, error) {
		_, _ = io.WriteString(spec.Log, `{"type":"system","session_id":"s1"}`+"\n")
		_, _ = io.WriteString(spec.Log, "test said: disk I/O error\n")
		return 1, nil
	}, newJob())

	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)
	got := env.api.done()[0]
	assert.False(t, got.limited)
	assert.Empty(t, got.blocked)
	assert.Empty(t, env.runner.Fault())
}

func TestTheNixStorePreflight(t *testing.T) {
	dir := t.TempDir()
	check := runner.NixStorePreflight(dir)
	require.NoError(t, check(context.Background()), "an unseeded store is fine")

	db := filepath.Join(dir, "var", "nix", "db")
	require.NoError(t, os.MkdirAll(db, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(db, "db.sqlite"), []byte("tiny"), 0o644))
	require.Error(t, check(context.Background()), "a database too short to read")

	require.NoError(t, os.WriteFile(filepath.Join(db, "db.sqlite"),
		[]byte("SQLite format 3\x00 more"), 0o644))
	require.NoError(t, check(context.Background()))

	require.Error(t, runner.NixStorePreflight(filepath.Join(dir, "gone"))(
		context.Background()))
}

func TestEnvironmentFailureSignatures(t *testing.T) {
	for text, want := range map[string]string{
		"Error: disk I/O error":          "the Nix store is unreadable",
		"nix: command not found":         "nix is missing in the VM",
		"write: no space left on device": "the VM cannot reach its image or mounts",
		"tests failed":                   "",
	} {
		assert.Equal(t, want, runner.EnvironmentFailure(text), text)
	}
}
