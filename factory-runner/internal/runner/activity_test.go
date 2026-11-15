package runner_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

type stubAwake struct {
	starts, stops int
	err           error
}

func (s *stubAwake) Start() (func(), error) {
	if s.err != nil {
		return nil, s.err
	}
	s.starts++
	return func() { s.stops++ }, nil
}

func awakeStub() runner.Awake { return &stubAwake{starts: 0, stops: 0, err: nil} }

func TestActivityHoldsTheMacAwakeWhileAnyJobRuns(t *testing.T) {
	awake := &stubAwake{starts: 0, stops: 0, err: nil}
	a := runner.NewActivity(awake)
	assert.True(t, a.Idle())

	require.NoError(t, a.Begin())
	require.NoError(t, a.Begin())
	assert.False(t, a.Idle())
	assert.Equal(t, 1, awake.starts, "one hold for two jobs")

	a.End()
	assert.False(t, a.Idle())
	assert.Equal(t, 0, awake.stops)
	a.End()
	assert.True(t, a.Idle())
	assert.Equal(t, 1, awake.stops)

	a.End() // extra End is harmless
	assert.True(t, a.Idle())

	require.NoError(t, a.Begin())
	assert.Equal(t, 2, awake.starts, "a later job holds again")
}

func TestActivityStillCountsAJobWhenTheHoldFails(t *testing.T) {
	awake := &stubAwake{starts: 0, stops: 0, err: errors.New("no caffeinate")}
	a := runner.NewActivity(awake)
	require.Error(t, a.Begin())
	assert.False(t, a.Idle())
	a.End()
	assert.True(t, a.Idle())
	assert.Equal(t, 0, awake.stops)
}

func TestCaffeinateStartsAndStopsItsProcess(t *testing.T) {
	var gotPID int
	stopped := false
	c := runner.NewCaffeinateWith(func(pid int) (func(), error) {
		gotPID = pid
		return func() { stopped = true }, nil
	})
	stop, err := c.Start()
	require.NoError(t, err)
	assert.Positive(t, gotPID)
	stop()
	assert.True(t, stopped)
}

func TestRealCaffeinateFailsCleanlyWithoutTheBinary(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := runner.NewCaffeinate().Start()
	require.Error(t, err)
}

func TestActivityRetriesTheHoldOnTheNextJob(t *testing.T) {
	awake := &stubAwake{starts: 0, stops: 0, err: errors.New("no caffeinate")}
	a := runner.NewActivity(awake)
	require.Error(t, a.Begin())

	awake.err = nil
	require.NoError(t, a.Begin(), "a second job tries again")
	assert.Equal(t, 1, awake.starts)
	a.End()
	assert.Equal(t, 0, awake.stops)
	a.End()
	assert.Equal(t, 1, awake.stops)
}

func TestActivityDrainIdle(t *testing.T) {
	now := time.Unix(1000, 0)
	a := runner.NewActivity(awakeStub()).WithClock(func() time.Time { return now })
	require.NoError(t, a.Begin())
	a.SetDraining(true)
	assert.False(t, a.DrainIdle())

	now = now.Add(runner.DrainTimeout)
	assert.True(t, a.DrainIdle(), "a leaked count must not hold the update")

	a.BeginVM()
	assert.False(t, a.DrainIdle(), "a running VM holds it")
	a.EndVM()
	assert.True(t, a.DrainIdle())

	a.SetDraining(false)
	assert.False(t, a.DrainIdle())
	assert.Contains(t, a.Busy(), "1 jobs")
}
