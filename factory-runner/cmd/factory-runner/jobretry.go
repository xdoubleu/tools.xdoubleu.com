package main

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

const (
	jobRetryFirst  = time.Minute
	jobRetryFactor = 2
	jobRetryMax    = 30 * time.Minute
)

// retryingJobs is the job loop of a runner whose jobs failed to start (a failed
// image pull, say): it builds them again with a growing pause until it can, then
// runs them, so jobs come back without a restart.
// jobRetrySleep is the pause between retries; tests replace it.
var jobRetrySleep = sleepFor //nolint:gochecknoglobals // tests replace it

type retryingJobs struct {
	build func(context.Context) (jobLoop, error)
	out   io.Writer
	sleep func(context.Context, time.Duration)
	// kick, when it fires, ends a pause early; control hears whether jobs are off.
	kick    <-chan struct{}
	control *runner.Control

	mu   sync.Mutex
	live jobLoop
}

// Reconcile frees leaked slots of the loop once it runs.
func (r *retryingJobs) Reconcile() int {
	r.mu.Lock()
	live := r.live
	r.mu.Unlock()
	if rec, ok := live.(interface{ Reconcile() int }); ok {
		return rec.Reconcile()
	}
	return 0
}

// pause waits d, or less when kicked.
func (r *retryingJobs) pause(ctx context.Context, d time.Duration) {
	if r.kick == nil {
		r.sleep(ctx, d)
		return
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		select {
		case <-r.kick:
			cancel()
		case <-ctx.Done():
		}
	}()
	r.sleep(ctx, d)
}

func (r *retryingJobs) Loop(ctx context.Context) error {
	backoff := jobRetryFirst
	for {
		r.pause(ctx, backoff)
		if err := ctx.Err(); err != nil {
			return nil //nolint:nilerr // stopping is not a failure
		}
		loop, err := r.build(ctx)
		if err == nil {
			fmt.Fprintln(r.out, "factory-runner: jobs are on")
			r.control.SetJobsOff("")
			r.mu.Lock()
			r.live = loop
			r.mu.Unlock()
			return loop.Loop(ctx) //nolint:wrapcheck // the loop's own result
		}
		fmt.Fprintln(r.out, "factory-runner: jobs are still off:", err)
		r.control.SetJobsOff(err.Error())
		backoff = min(backoff*jobRetryFactor, jobRetryMax)
	}
}

func sleepFor(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
	case <-ctx.Done():
	}
}
