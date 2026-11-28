package runner_test

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
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
	assert.True(t, strings.HasPrefix(done[0].outcome, "Runner environment fault: "+
		"the Nix store is unreadable; task requeued."), done[0].outcome)
	assert.NotContains(t, done[0].outcome, "usage limit")
	assert.Equal(t, "the Nix store is unreadable; re-attach the store volume",
		env.runner.Fault())

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

// setupDied is a run whose harness started, then died on the VM's Nix setup
// before doing any work.
func setupDied(line string) func(context.Context, runner.SandboxSpec) (int, error) {
	return func(_ context.Context, spec runner.SandboxSpec) (int, error) {
		_, _ = io.WriteString(spec.Log, `{"type":"system","session_id":"s1"}`+"\n")
		_, _ = io.WriteString(spec.Log, line+"\n")
		return 126, nil
	}
}

func TestASetupFailureAfterTheHarnessStartedIsTheEnvironments(t *testing.T) {
	for _, line := range []string{
		"env: 'nix': No such device or address",
		"wardcase: seeding /nix from /opt/nix-seed failed",
	} {
		env := newEnv(t, setupDied(line), newJob())

		_, err := env.runner.Next(context.Background())
		require.NoError(t, err)
		got := env.api.done()[0]
		assert.True(t, got.limited, line)
		assert.Equal(t, "Runner environment: the Nix store is unreadable", got.blocked)
	}
}

func TestASetupSignatureAfterTheAgentCommittedIsNotTheEnvironments(t *testing.T) {
	env := newEnv(t, func(_ context.Context, spec runner.SandboxSpec) (int, error) {
		commitFile(spec.CloneDir, "a.txt", "x")
		_, _ = io.WriteString(spec.Log, `{"type":"system","session_id":"s1"}`+"\n")
		_, _ = io.WriteString(spec.Log, "env: 'nix': No such device or address\n")
		return 1, nil
	}, newJob())

	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)
	assert.False(t, env.api.done()[0].limited)
	assert.Empty(t, env.runner.Fault())
}

func writeDB(t *testing.T, dir, body string) {
	t.Helper()
	db := filepath.Join(dir, "var", "nix", "db")
	require.NoError(t, os.MkdirAll(db, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(db, "db.sqlite"), []byte(body), 0o644))
}

func TestTheNixStorePreflight(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nix")
	require.NoError(t, os.Mkdir(dir, 0o755))
	check := runner.NixStorePreflight(dir)
	require.NoError(t, check(context.Background()), "an unseeded store is fine")

	writeDB(t, dir, "tiny")
	require.Error(t, check(context.Background()), "a database too short to read")

	writeDB(t, dir, "SQLite format 3\x00 more")
	require.NoError(t, check(context.Background()), "a store/ not there yet")
	assert.NoFileExists(t, dir+".seeded")

	require.NoError(t, os.MkdirAll(filepath.Join(dir, "store", "abc-hello"), 0o755))
	require.NoError(t, check(context.Background()))
	assert.FileExists(t, dir+".seeded")

	require.Error(t, runner.NixStorePreflight(filepath.Join(dir, "gone"))(
		context.Background()))
}

func TestASeededStoreThatListsEmptyIsADeadMount(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nix")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "store", "abc-hello"), 0o755))
	writeDB(t, dir, "SQLite format 3\x00 more")
	check := runner.NixStorePreflight(dir)
	require.NoError(t, check(context.Background()))

	// The mount drops: the directory lists empty and nothing in it opens.
	require.NoError(t, os.RemoveAll(filepath.Join(dir, "var")))
	require.Error(t, check(context.Background()), "the database is gone")

	writeDB(t, dir, "SQLite format 3\x00 more")
	require.NoError(t, os.RemoveAll(filepath.Join(dir, "store", "abc-hello")))
	require.ErrorContains(t, check(context.Background()), "store/ is empty")
}

func TestEnvironmentFailureSignatures(t *testing.T) {
	for text, want := range map[string]string{
		"Error: disk I/O error":  "the Nix store is unreadable",
		"nix: command not found": "nix is missing in the VM",
		"write: no space left on device": "the VM cannot reach its image or " +
			"mounts",
		"wardcase: seeding /nix from /opt/nix-seed failed": "the Nix store is unreadable",
		"tests failed": "",
	} {
		assert.Equal(t, want, runner.EnvironmentFailure(text, false), text)
	}
	assert.Empty(t, runner.EnvironmentFailure("Error: disk I/O error", true),
		"a test's I/O error is not the setup's")
	assert.Equal(t, "the Nix store is unreadable",
		runner.EnvironmentFailure("Device not configured", true))
}

func TestADroppedStoreStopsTheRunnersVMsOnce(t *testing.T) {
	env := newEnv(t, deadNix, newJob())
	var stopped [][]string
	env.runner.StopVMs = func(_ context.Context, jobs []string) []string {
		stopped = append(stopped, jobs)
		return nil
	}

	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)

	assert.Equal(t, [][]string{{newJob().ID}}, stopped,
		"the job's own VM is stopped before the volume is attached again")
}

func TestAnotherEnvironmentFaultStopsNoVMs(t *testing.T) {
	env := newEnv(t, setupDied("env: 'nix': No such file or directory"), newJob())
	env.runner.StopVMs = func(context.Context, []string) []string {
		t.Fatal("no VM is stopped for a missing nix")
		return nil
	}

	_, err := env.runner.Next(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "nix is missing in the VM", env.runner.Fault())
}

func TestFilePressureHoldsClaimsAboveSeventyPercent(t *testing.T) {
	at := func(open uint32) func() (uint32, uint32, error) {
		return func() (uint32, uint32, error) { return open, 1000, nil }
	}

	require.NoError(t, runner.FilePressurePreflight(at(700))(context.Background()))
	err := runner.FilePressurePreflight(at(701))(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "701 of 1000 system files open (kern.maxfiles)")
	require.NoError(t, runner.FilePressurePreflight(func() (uint32, uint32, error) {
		return 0, 0, errors.New("no sysctl")
	})(context.Background()), "an unknown count holds nothing")
}

func TestPreflightsFailOnTheFirstFailingCheck(t *testing.T) {
	var ran []string
	check := func(name string, err error) func(context.Context) error {
		return func(context.Context) error {
			ran = append(ran, name)
			return err
		}
	}

	err := runner.Preflights(check("a", nil), check("b", errors.New("b")),
		check("c", nil))(context.Background())

	require.EqualError(t, err, "b")
	assert.Equal(t, []string{"a", "b"}, ran)
	require.NoError(t, runner.Preflights()(context.Background()))
}

func TestSystemFilesReadsThisMachinesCounts(t *testing.T) {
	open, limit, err := runner.SystemFiles()
	require.NoError(t, err)
	assert.LessOrEqual(t, open, limit)
}
