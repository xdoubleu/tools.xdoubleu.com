package runner_test

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

func TestStopSurvivesARestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "paused")
	c := runner.NewControl(path, "v1", nil)
	assert.False(t, c.Paused())

	require.NoError(t, c.SetPaused(true))
	assert.True(t, runner.NewControl(path, "v1", nil).Paused())

	require.NoError(t, c.SetPaused(false))
	require.NoError(t, c.SetPaused(false), "starting twice is fine")
	assert.False(t, runner.NewControl(path, "v1", nil).Paused())
}

func TestStopFailsWhereTheFlagCannotBeWritten(t *testing.T) {
	dir := t.TempDir()
	c := runner.NewControl(filepath.Join(dir, "paused", "x"), "v", nil)
	flag := runner.NewControl(filepath.Join(dir, "paused"), "v", nil)
	require.NoError(t, flag.SetPaused(true))
	require.Error(t, c.SetPaused(true))
	assert.False(t, c.Paused())
}

func TestStoppedRunnerKeepsBeatingUntilItsJobEnds(t *testing.T) {
	activity := runner.NewActivity(awakeStub())
	c := runner.NewControl(filepath.Join(t.TempDir(), "paused"), "v", activity)
	require.NoError(t, activity.Begin())
	require.NoError(t, c.SetPaused(true))

	assert.False(t, c.Quiet())
	assert.Equal(t, "Stopping after the current job", c.Snapshot().Line())

	activity.End()
	assert.True(t, c.Quiet())
	assert.Equal(t, "Stopped", c.Snapshot().Line())
}

func TestStatusLines(t *testing.T) {
	assert.Equal(t, "Idle", status(func(*runner.Status) {}).Line())
	busy := status(func(s *runner.Status) { s.Busy = true })
	assert.Equal(t, "Running a job", busy.Line())
}

func TestARequestedCheckSkipsTheHourlyWait(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	control := runner.NewControl(filepath.Join(t.TempDir(), "paused"), "v", nil)
	upd := newFakeUpdater(false, nil)
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	hb := newScripted(func(call int) error {
		if call == 1 {
			upd.waitCalled()
			control.RequestCheck()
		}
		if call == 4 {
			cancel()
		}
		return nil
	}, cancel)
	require.NoError(t, runner.RunForTest(ctx, hb, "frt_x", newQuiet(),
		runner.Options{Updater: upd, Activity: nil, Control: control},
		func(context.Context, time.Duration) { time.Sleep(5 * time.Millisecond) },
		func() time.Time { return now }))
	assert.Equal(t, 2, int(upd.calls.Load()))
}

func TestMenuLines(t *testing.T) {
	now := time.Date(2026, 11, 1, 12, 0, 10, 0, time.UTC)
	beat := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)

	l := status(func(s *runner.Status) {
		s.LastHeartbeat, s.Update = beat, "no newer release"
	}).MenuLines("965ebd6df00af8dd973a92e2d9f3b93b8d3c0162", now)
	assert.Equal(t, runner.MenuLines{
		Header:    "Factory Runner 965ebd6",
		Status:    "Idle",
		Heartbeat: "Last heartbeat 10s ago",
		Update:    "Update: no newer release",
	}, l)

	l = status(func(s *runner.Status) { s.Paused = true }).MenuLines("dev", now)
	assert.Equal(t, "Factory Runner dev", l.Header)
	assert.Equal(t, "Offline on the board", l.Heartbeat)
	assert.Equal(t, "No update check yet", l.Update)

	fresh := status(func(*runner.Status) {}).MenuLines("dev", now)
	assert.Equal(t, "No heartbeat yet", fresh.Heartbeat)
}

// status builds a zero Status and lets set fill in what a case needs.
func status(set func(*runner.Status)) runner.Status {
	s := runner.Status{
		Version: "", Paused: false, Busy: false, LastHeartbeat: time.Time{}, Update: "",
		Mode: "",
	}
	set(&s)
	return s
}

func TestARequestedCheckWaitsForAJobToEnd(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	control := runner.NewControl(filepath.Join(t.TempDir(), "paused"), "v", nil)
	activity := runner.NewActivity(awakeStub())
	require.NoError(t, activity.Begin())
	upd := newFakeUpdater(false, nil)
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	control.RequestCheck()
	hb := newScripted(func(call int) error {
		if call == 3 {
			activity.End()
		}
		if call == 6 {
			upd.waitCalled()
			cancel()
		}
		return nil
	}, cancel)
	require.NoError(t, runner.RunForTest(ctx, hb, "frt_x", newQuiet(),
		runner.Options{Updater: upd, Activity: activity, Control: control},
		func(context.Context, time.Duration) { time.Sleep(time.Millisecond) },
		func() time.Time { return now }))
	assert.GreaterOrEqual(t, int(upd.calls.Load()), 1)
}

func TestARequestedCheckIsRefusedWhileStopped(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	control := runner.NewControl(filepath.Join(t.TempDir(), "paused"), "v", nil)
	require.NoError(t, control.SetPaused(true))
	control.RequestCheck()
	upd := newFakeUpdater(false, nil)
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	// A stopped idle runner never heartbeats, so the loop's sleep ends the test.
	hb := newScripted(nil, cancel)
	require.NoError(t, runner.RunForTest(ctx, hb, "frt_x", newQuiet(),
		runner.Options{Updater: upd, Activity: nil, Control: control},
		func(context.Context, time.Duration) { cancel() },
		func() time.Time { return now }))
	assert.Zero(t, upd.calls.Load())
	assert.Contains(t, control.Snapshot().Update, "stopped")
}

func TestARequestedCheckCutsAHeldHeartbeatShort(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	control := runner.NewControl(filepath.Join(t.TempDir(), "paused"), "v", nil)
	upd := newFakeUpdater(false, nil)
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	held := make(chan struct{})
	hb := &holdingBeat{held: held, once: sync.Once{}}
	go func() {
		<-held
		control.RequestCheck()
	}()
	done := make(chan error, 1)
	go func() {
		done <- runner.RunForTest(ctx, hb, "frt_x", newQuiet(),
			runner.Options{Updater: upd, Activity: nil, Control: control},
			func(context.Context, time.Duration) { time.Sleep(time.Millisecond) },
			func() time.Time { return now })
	}()
	upd.waitCalled()
	cancel()
	require.NoError(t, <-done)
}

// holdingBeat blocks like the server's held heartbeat until its context ends.
type holdingBeat struct {
	held chan struct{}
	once sync.Once
}

func (h *holdingBeat) Heartbeat(ctx context.Context, _ string) error {
	h.once.Do(func() { close(h.held) })
	<-ctx.Done()
	return ctx.Err()
}
