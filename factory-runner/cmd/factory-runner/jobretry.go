package main

import (
	"context"
	"fmt"
	"io"
	"time"
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
}

func (r *retryingJobs) Loop(ctx context.Context) error {
	backoff := jobRetryFirst
	for {
		r.sleep(ctx, backoff)
		if err := ctx.Err(); err != nil {
			return nil //nolint:nilerr // stopping is not a failure
		}
		loop, err := r.build(ctx)
		if err == nil {
			fmt.Fprintln(r.out, "factory-runner: jobs are on")
			return loop.Loop(ctx) //nolint:wrapcheck // the loop's own result
		}
		fmt.Fprintln(r.out, "factory-runner: jobs are still off:", err)
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
