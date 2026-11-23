package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

func TestStatusSaysWhyTheRunnerDoesOrDoesNotClaim(t *testing.T) {
	e := newEnv(t)
	require.Error(t, e.app.status(), "no status written yet")

	control := runner.NewControl(filepath.Join(e.app.home, "paused"), "v9", nil)
	control.SetJobsOff("pull failed: no space left")
	control.SetClaimSources(runner.ClaimSources{
		Max: func() int { return 2 }, Jobs: func() []string { return []string{"j1"} },
		LimitUntil: nil, Fault: nil,
	})
	control.RecordQueued(2)
	require.NoError(t, runner.WriteStatus(e.app.home, control.Report()))

	require.NoError(t, e.app.status())
	out := e.out.String()
	assert.Contains(t, out,
		"Holding: jobs off: pull failed: no space left (1 of 2 slots used)")
	assert.Contains(t, out, "j1")
	assert.Contains(t, out, "2 waiting")
}

func TestStatusIsAKnownCommand(t *testing.T) {
	e := newEnv(t)
	assert.Equal(t, 1, e.app.run(context.Background(), []string{"status"}))
	assert.Contains(t, usage, "status")
}

func TestAKickEndsTheJobRetryPauseAtOnce(t *testing.T) {
	kick := make(chan struct{}, 1)
	tries := 0
	loop := &retryingJobs{
		control: nil, mu: sync.Mutex{}, live: nil,
		build: func(context.Context) (jobLoop, error) {
			tries++
			if tries == 1 {
				return nil, errors.New("pull failed")
			}
			return funcLoop(func(context.Context) error { return nil }), nil
		},
		out: &bytes.Buffer{},
		sleep: func(ctx context.Context, _ time.Duration) {
			<-ctx.Done()
		},
		kick: kick,
	}
	kick <- struct{}{}
	done := make(chan error, 1)
	go func() { done <- loop.Loop(context.Background()) }()
	select {
	case <-time.After(time.Second):
		t.Fatal("the first pause was not cut short")
	case <-time.After(50 * time.Millisecond):
	}
	kick <- struct{}{}
	require.NoError(t, <-done)
	assert.Equal(t, 2, tries)
}

type reconcileLoop struct{ funcLoop }

func (reconcileLoop) Reconcile() int { return 3 }

func TestARetryingLoopReconcilesTheLoopItBuilt(t *testing.T) {
	r := &retryingJobs{
		build: nil, out: nil, sleep: nil, kick: nil, control: nil,
		mu: sync.Mutex{}, live: nil,
	}
	assert.Zero(t, r.Reconcile())
	r.live = reconcileLoop{funcLoop(blockUntilStopped)}
	assert.Equal(t, 3, r.Reconcile())
}

func TestWatchWritesTheStatusFileUntilStopped(t *testing.T) {
	e := newEnv(t)
	e.app.log = slog.New(slog.NewTextHandler(io.Discard, nil))
	e.app.watch(context.Background(), funcLoop(blockUntilStopped))()

	e.app.control = runner.NewControl(filepath.Join(e.app.home, "paused"), "v1", nil)
	stop := e.app.watch(context.Background(), reconcileLoop{funcLoop(blockUntilStopped)})
	require.Eventually(t, func() bool {
		r, err := runner.ReadStatus(e.app.home)
		return err == nil && r.Version == "v1"
	}, 5*time.Second, 10*time.Millisecond)
	stop()
}
