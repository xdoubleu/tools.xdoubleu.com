package runner_test

import (
	"bytes"
	"context"
	"log/slog"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

type dogEnv struct {
	dog     *runner.Watchdog
	state   runner.ClaimState
	now     time.Time
	freed   int
	kicks   atomic.Int32
	restart atomic.Int32
	logs    *bytes.Buffer
}

func newDog() *dogEnv {
	e := &dogEnv{
		dog: nil, freed: 0, kicks: atomic.Int32{}, restart: atomic.Int32{},
		now:  time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC),
		logs: &bytes.Buffer{},
		state: runner.ClaimState{
			Claiming: true, Reason: "", Owner: false, Draining: false, JobsOff: "",
			Running: 0, Max: 2, Jobs: nil, Queued: 1,
		},
	}
	e.dog = &runner.Watchdog{
		State:     func() runner.ClaimState { return e.state },
		Reconcile: func() int { n := e.freed; e.freed = 0; return n },
		KickJobs:  func() { e.kicks.Add(1) },
		Restart:   func() { e.restart.Add(1) },
		Log:       slog.New(slog.NewTextHandler(e.logs, nil)),
		Now:       func() time.Time { return e.now },
	}
	return e
}

func (e *dogEnv) advance(d time.Duration) bool {
	e.now = e.now.Add(d)
	return e.dog.Check()
}

func TestWorkWaitingLessThanTheStallWindowIsLeftAlone(t *testing.T) {
	e := newDog()
	assert.False(t, e.advance(0))
	assert.False(t, e.advance(9*time.Minute))
	assert.Empty(t, e.logs.String())
}

func TestWorkWaitingPastTheWindowIsLoggedWithItsReason(t *testing.T) {
	e := newDog()
	e.state.Claiming, e.state.Reason = false, "restarting after an update"
	e.state.Running, e.state.Jobs = 1, []string{"job-a"}
	e.advance(0)

	assert.True(t, e.advance(11*time.Minute))

	out := e.logs.String()
	assert.Contains(t, out, "stalled")
	assert.Contains(t, out, "restarting after an update")
	assert.Contains(t, out, "job-a")
	assert.Zero(t, e.restart.Load(), "only a stuck drain restarts")
}

func TestAClaimingRunnerThatGetsNothingIsStillReported(t *testing.T) {
	e := newDog()
	e.advance(0)
	assert.True(t, e.advance(11*time.Minute))
	assert.Contains(t, e.logs.String(), "hands out nothing")
}

func TestARecoveredRunnerResetsTheWindow(t *testing.T) {
	e := newDog()
	e.advance(0)
	e.advance(8 * time.Minute)
	e.state.Queued = 0
	assert.False(t, e.advance(time.Minute))
	e.state.Queued = 1
	assert.False(t, e.advance(5*time.Minute), "the wait starts again")
	assert.True(t, e.advance(11*time.Minute))
}

func TestWhatTheOwnerStoppedIsNotAStall(t *testing.T) {
	e := newDog()
	e.state.Claiming, e.state.Owner, e.state.Reason = false, true, "stopped from the menu"
	e.advance(0)
	assert.False(t, e.advance(time.Hour))
}

func TestAFullRunnerIsNotStalled(t *testing.T) {
	e := newDog()
	e.state.Running, e.state.Claiming, e.state.Reason = 2, false, "all slots busy"
	e.advance(0)
	assert.False(t, e.advance(time.Hour))
}

func TestJobsThatAreOffAreRetriedAndLeakedSlotsFreed(t *testing.T) {
	e := newDog()
	e.state.Claiming, e.state.Reason = false, "jobs off: pull failed"
	e.state.JobsOff = "pull failed"
	e.freed = 1
	e.advance(0)

	assert.True(t, e.advance(11*time.Minute))

	assert.Equal(t, int32(1), e.kicks.Load())
	assert.Contains(t, e.logs.String(), "freed leaked job slots")
	assert.False(t, e.advance(5*time.Minute), "not again within the window")
	assert.True(t, e.advance(6*time.Minute), "again in the next window")
	assert.Equal(t, int32(2), e.kicks.Load())
}

func TestAStuckDrainRestartsTheRunner(t *testing.T) {
	e := newDog()
	e.state.Claiming, e.state.Draining, e.state.Reason = false, true, "draining"
	e.state.Running = 0
	e.advance(0)
	assert.True(t, e.advance(11*time.Minute))
	assert.Equal(t, int32(1), e.restart.Load())
}

func TestAFreedSlotGivesTheDrainAnotherWindowBeforeARestart(t *testing.T) {
	e := newDog()
	e.state.Claiming, e.state.Draining, e.state.Reason = false, true, "draining"
	e.freed = 1
	e.advance(0)
	assert.True(t, e.advance(11*time.Minute))
	assert.Zero(t, e.restart.Load())
}

func TestWatchdogUsesTheRealClockWithoutOne(t *testing.T) {
	e := newDog()
	e.dog.Now = nil
	e.state.Queued = 0
	assert.False(t, e.dog.Check())
}

func TestClaimStateNamesWhyAndTheStatusFileRoundTrips(t *testing.T) {
	home := t.TempDir()
	activity := runner.NewActivity(awakeStub())
	c := runner.NewControl(filepath.Join(home, "paused"), "v1", activity)
	c.SetClaimSources(runner.ClaimSources{
		Max:        func() int { return 2 },
		Jobs:       func() []string { return []string{"b", "a"} },
		LimitUntil: nil, Fault: nil,
	})
	c.RecordQueued(3)

	st := c.ClaimState()
	assert.Equal(t, []string{"a", "b"}, st.Jobs)
	assert.Equal(t, "Holding: all slots busy (2 of 2 slots used)", st.Line())
	assert.Equal(t, 3, st.Queued)

	c.SetClaimSources(runner.ClaimSources{
		Max: func() int { return 2 }, Jobs: func() []string { return []string{"a"} },
		LimitUntil: nil, Fault: nil,
	})
	assert.Equal(t, "Claiming (1 of 2 slots used)", c.ClaimState().Line())

	c.SetJobsOff("pull: no space left")
	assert.Equal(t, "Holding: jobs off: pull: no space left (1 of 2 slots used)",
		c.ClaimState().Line())
	c.SetJobsOff("")
	c.SetDraining(true)
	assert.Contains(t, c.ClaimState().Reason, "draining")
	c.SetRestarting()
	assert.Contains(t, c.ClaimState().Reason, "restarting")
	require.NoError(t, c.SetPaused(true))
	assert.Equal(t, "stopped from the menu", c.ClaimState().Line())
	assert.True(t, c.ClaimState().Owner)

	require.NoError(t, runner.WriteStatus(home, c.Report()))
	got, err := runner.ReadStatus(home)
	require.NoError(t, err)
	assert.Equal(t, "v1", got.Version)
	assert.Equal(t, "stopped from the menu", got.Claim.Reason)

	out := got.Render(got.Written.Add(time.Second))
	assert.Contains(t, out, "Claims:     stopped from the menu")
	assert.Contains(t, out, "Queued:     3 waiting")
	assert.Contains(t, got.Render(got.Written.Add(time.Hour)), "not running")
}

func TestClaimStateReasonsInOrder(t *testing.T) {
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	c := runner.NewControl(filepath.Join(t.TempDir(), "paused"), "v", nil)
	assert.Equal(t, runner.QueuedUnknown, c.ClaimState().Queued)

	c.SetUnpaired()
	assert.Equal(t, "not paired", c.ClaimState().Reason)
	c.MarkPaired()

	c.SetModeSource(func() string { return runner.ModeOff })
	assert.Equal(t, "mode is off", c.ClaimState().Reason)
	c.SetModeSource(func() string { return runner.ModeClaude })

	c.SetClaimSources(runner.ClaimSources{
		Max: func() int { return 0 }, Jobs: nil,
		LimitUntil: func() time.Time { return time.Now().Add(time.Hour) },
		Fault:      nil,
	})
	assert.Contains(t, c.ClaimState().Reason, "usage limit until")
	_ = now
	assert.Contains(t, c.Snapshot().MenuLines("abcdefghij", now).Status, "Holding")
	assert.Equal(t, 1, c.ClaimState().Max)
}

func TestRestartAndKickRequestsAreDelivered(t *testing.T) {
	c := runner.NewControl(filepath.Join(t.TempDir(), "paused"), "v", nil)
	c.KickJobs()
	c.KickJobs()
	select {
	case <-c.JobsKicked():
	case <-time.After(time.Second):
		t.Fatal("no kick")
	}
}

func TestADaemonEndsWithARestartWhenTheWatchdogAsks(t *testing.T) {
	c := runner.NewControl(filepath.Join(t.TempDir(), "paused"), "v", nil)
	c.RequestRestart()
	hb := newScripted(func(int) error { return nil }, func() {})
	err := runner.Run(t.Context(), hb, "frt_x", newQuiet(), options(nil, nil, c))
	assert.ErrorIs(t, err, runner.ErrRestart)
}

type queuedBeat struct{ calls int }

func (q *queuedBeat) Heartbeat(context.Context, string) error {
	q.calls++
	return nil
}
func (q *queuedBeat) Queued() int { return 4 }

func TestTheDaemonRecordsTheServersQueuedCount(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	c := runner.NewControl(filepath.Join(t.TempDir(), "paused"), "v", nil)
	hb := &queuedBeat{calls: 0}
	done := make(chan error, 1)
	go func() {
		opts := options(nil, nil, c)
		opts.Sleep = func(context.Context, time.Duration) { cancel() }
		done <- runner.Run(ctx, hb, "frt_x", newQuiet(), opts)
	}()
	require.NoError(t, <-done)
	assert.Equal(t, 4, c.ClaimState().Queued)
}

func TestActivityReconcileDropsACountNoJobBacks(t *testing.T) {
	a := runner.NewActivity(awakeStub())
	require.NoError(t, a.Begin())
	require.NoError(t, a.Begin())
	assert.Zero(t, a.Reconcile(2))
	assert.Equal(t, 2, a.Reconcile(0))
	assert.True(t, a.Idle())
}

func TestAVMFaultHoldsClaimsAndShowsOnTheStatus(t *testing.T) {
	c := runner.NewControl(filepath.Join(t.TempDir(), "paused"), "v1",
		runner.NewActivity(awakeStub()))
	c.SetClaimSources(runner.ClaimSources{
		Max: func() int { return 1 }, Jobs: nil, LimitUntil: nil,
		Fault: func() string { return "no image" },
	})
	st := c.ClaimState()
	assert.False(t, st.Claiming)
	assert.Equal(t, "Holding: runner fault: no image (0 of 1 slots used)", st.Line())
}
