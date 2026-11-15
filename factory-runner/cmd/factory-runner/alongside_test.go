package main

import (
	"context"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

func always(err error) func(context.Context) error {
	return func(context.Context) error { return err }
}

func untilStopped(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func TestTheMostTellingErrorWinsWhicheverLoopEndsFirst(t *testing.T) {
	boom := errors.New("boom")
	for name, tc := range map[string]struct {
		first, second error
		want          error
	}{
		"revoked beats restart": {runner.ErrRestart, runner.ErrRevoked, runner.ErrRevoked},
		"revoked beats other":   {boom, runner.ErrRevoked, runner.ErrRevoked},
		"restart beats other":   {boom, runner.ErrRestart, runner.ErrRestart},
		"other when nothing":    {nil, boom, boom},
		"nil when both nil":     {nil, nil, nil},
	} {
		for _, swap := range []bool{false, true} {
			a, b := always(tc.first), always(tc.second)
			if swap {
				// The other order: the second loop finishes first.
				a = func(context.Context) error {
					time.Sleep(20 * time.Millisecond)
					return tc.first
				}
			}
			got := alongside(context.Background(), a, b, nil, func() bool { return true },
				time.Millisecond)
			assert.Equal(t, tc.want, got, name)
		}
	}
}

func TestARestartWaitsForARunningJobBeforeStoppingTheLoop(t *testing.T) {
	var running atomic.Bool
	running.Store(true)
	var stoppedWhileRunning atomic.Bool
	loop := func(ctx context.Context) error {
		<-ctx.Done()
		if running.Load() {
			stoppedWhileRunning.Store(true)
		}
		return nil
	}
	go func() {
		time.Sleep(60 * time.Millisecond)
		running.Store(false)
	}()

	err := alongside(context.Background(), always(runner.ErrRestart), loop, nil,
		func() bool { return !running.Load() }, 5*time.Millisecond)

	assert.ErrorIs(t, err, runner.ErrRestart)
	assert.False(t, stoppedWhileRunning.Load(), "the job was left to finish")
}

func TestAShutdownDuringTheWaitStopsItAtOnce(t *testing.T) {
	ctx, stop := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		stop()
	}()
	start := time.Now()

	err := alongside(ctx, always(runner.ErrRestart), untilStopped, nil,
		func() bool { return false }, time.Hour)

	assert.ErrorIs(t, err, runner.ErrRestart)
	assert.Less(t, time.Since(start), 5*time.Second)
}

func TestARestartKeepsHeartbeatingAndHoldsClaimsUntilIdle(t *testing.T) {
	e := newEnv(t)
	e.app.control = runner.NewControl(filepath.Join(t.TempDir(), "pause"), "v", nil)
	var beats atomic.Int32
	e.app.newBeat = func(string) pairHeartbeater { return countBeat{&beats} }
	var running atomic.Bool
	running.Store(true)
	go func() {
		time.Sleep(50 * time.Millisecond)
		running.Store(false)
	}()
	onRestart := func(ctx context.Context) { e.app.awaitRestart(ctx, "s", "tok") }

	err := alongside(context.Background(), always(runner.ErrRestart), untilStopped,
		onRestart, func() bool { return !running.Load() }, 5*time.Millisecond)

	assert.ErrorIs(t, err, runner.ErrRestart)
	assert.True(t, e.app.holdClaims(), "no claims once a restart is pending")
	assert.Positive(t, beats.Load(), "heartbeats continue while waiting")
}

type countBeat struct{ n *atomic.Int32 }

func (c countBeat) Heartbeat(context.Context, string) error {
	c.n.Add(1)
	time.Sleep(5 * time.Millisecond)
	return nil
}

func (countBeat) Pair(context.Context, string) (string, string, error) {
	return "", "", nil
}
