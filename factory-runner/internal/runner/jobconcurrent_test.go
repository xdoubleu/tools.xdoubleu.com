package runner_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

// jobNo is a job that differs from the others in id, task and branch.
func jobNo(n int) *runner.ClaimedJob {
	j := newJob()
	j.ID = fmt.Sprintf("11111111-2222-3333-4444-%012d", n)
	j.TaskID = fmt.Sprintf("T-%d", n)
	j.Branch = fmt.Sprintf("agent/%d-work", n)
	return j
}

func (f *fakeAPI) queued() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.jobs)
}

// startLoop runs the claim loop until the test ends.
func startLoop(t *testing.T, r *runner.JobRunner) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- r.Loop(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			assert.NoError(t, err)
		case <-time.After(10 * time.Second):
			t.Error("the loop did not stop")
		}
	})
}

func waitCompleted(t *testing.T, env *jobEnv, n int) {
	t.Helper()
	for range n {
		select {
		case <-env.api.completes:
		case <-time.After(10 * time.Second):
			t.Fatal("jobs did not finish")
		}
	}
}

func TestTwoJobsRunAtTheSameTimeEachInItsOwnSandbox(t *testing.T) {
	var arrived atomic.Int32
	both := make(chan struct{})
	env := newEnv(t, func(ctx context.Context, spec runner.SandboxSpec) (int, error) {
		if arrived.Add(1) == 2 {
			close(both)
		}
		select {
		case <-both:
		case <-time.After(5 * time.Second):
			return 1, fmt.Errorf("the other job never started")
		}
		return writes("Did it.", 0)(ctx, spec)
	}, jobNo(1), jobNo(2))
	env.runner.MaxJobs = func() int { return 2 }
	activity := runner.NewActivity(awakeStub())
	env.runner.Activity = activity

	startLoop(t, env.runner)
	waitCompleted(t, env, 2)

	done := env.api.done()
	require.Len(t, done, 2)
	for _, d := range done {
		assert.NotContains(t, d.outcome, "never started")
	}
	env.sandbox.mu.Lock()
	defer env.sandbox.mu.Unlock()
	require.Len(t, env.sandbox.specs, 2)
	assert.NotEqual(t, env.sandbox.specs[0].CloneDir, env.sandbox.specs[1].CloneDir)
	assert.NotEqual(t, env.sandbox.specs[0].OutcomeDir, env.sandbox.specs[1].OutcomeDir)
	assert.Eventually(t, activity.Idle, time.Second, time.Millisecond,
		"the Mac may sleep once the last job ends")
}

func TestARunnerNeverRunsMoreJobsThanItsCap(t *testing.T) {
	var running, peak atomic.Int32
	env := newEnv(t, func(ctx context.Context, spec runner.SandboxSpec) (int, error) {
		n := running.Add(1)
		defer running.Add(-1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		time.Sleep(400 * time.Millisecond)
		return writes("Did it.", 0)(ctx, spec)
	}, jobNo(1), jobNo(2), jobNo(3))
	env.runner.MaxJobs = func() int { return 2 }

	startLoop(t, env.runner)
	waitCompleted(t, env, 3)

	assert.Equal(t, int32(2), peak.Load())
}

func TestAUsageLimitStopsEveryClaimUntilItLifts(t *testing.T) {
	reset := time.Now().Add(2 * time.Hour).Unix()
	env := newEnv(t,
		limitSandbox([]string{rejectedAt(reset)}, "", 1), jobNo(1))
	env.runner.MaxJobs = func() int { return 2 }

	startLoop(t, env.runner)
	waitCompleted(t, env, 1)
	env.api.mu.Lock()
	env.api.jobs = append(env.api.jobs, jobNo(2))
	env.api.mu.Unlock()

	time.Sleep(2500 * time.Millisecond)
	assert.Equal(t, 1, env.api.queued(), "no job is claimed while the limit holds")
}
