package factory_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
)

func (e jobEnv) claimUpTo(t *testing.T, maxJobs int32) *factoryv1.ClaimedJob {
	t.Helper()
	resp, err := e.runner.ClaimJob(bearer(e.token),
		&factoryv1.ClaimJobRequest{MaxJobs: maxJobs})
	require.NoError(t, err)
	return resp.Job
}

// claimConcurrently makes n claims at once and returns the ids of the jobs won.
func (e jobEnv) claimConcurrently(t *testing.T, n int, maxJobs int32) []string {
	t.Helper()
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		claimed []string
	)
	for range n {
		wg.Go(func() {
			if job := e.claimUpTo(t, maxJobs); job != nil {
				mu.Lock()
				claimed = append(claimed, job.Id)
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	return claimed
}

func TestARunnerRunsAsManyJobsAsItAsksFor(t *testing.T) {
	env := browserJobEnv(t)
	for _, title := range []string{"a", "b", "c"} {
		env.run(t, env.newTask(t, title).Id, "")
	}

	first := env.claimUpTo(t, 2)
	second := env.claimUpTo(t, 2)
	require.NotNil(t, first)
	require.NotNil(t, second)
	assert.NotEqual(t, first.Id, second.Id)
	assert.Nil(t, env.claimUpTo(t, 2), "the cap holds at two")

	_, err := env.runner.CompleteJob(bearer(env.token),
		&factoryv1.CompleteJobRequest{JobId: first.Id, Succeeded: true})
	require.NoError(t, err)
	assert.NotNil(t, env.claimUpTo(t, 2), "a free slot is claimed again")
}

func TestConcurrentClaimsNeverTakeTheSameJob(t *testing.T) {
	env := browserJobEnv(t)
	const jobs = 6
	for range jobs {
		env.run(t, env.newTask(t, "race").Id, "")
	}

	claimed := env.claimConcurrently(t, 12, jobs)

	seen := map[string]struct{}{}
	for _, id := range claimed {
		seen[id] = struct{}{}
	}
	assert.Len(t, claimed, jobs, "every queued job is taken")
	assert.Len(t, seen, jobs, "no job is taken twice")
}

func TestConcurrentClaimsRespectTheRunnersCap(t *testing.T) {
	env := browserJobEnv(t)
	for range 6 {
		env.run(t, env.newTask(t, "cap").Id, "")
	}
	assert.Len(t, env.claimConcurrently(t, 10, 2), 2)
}
