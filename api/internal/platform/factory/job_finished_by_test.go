package factory_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
)

func finishedJob(t *testing.T, env jobEnv, taskID string) *factoryv1.Job {
	t.Helper()
	got := env.task(t, taskID)
	require.NotEmpty(t, got.Jobs)
	return got.Jobs[0]
}

func TestAJobThatContinuedOnTheFallbackShowsWhatFinishedIt(t *testing.T) {
	env := browserJobEnv(t)
	task := env.newTask(t, "continues on the fallback")
	env.run(t, task.Id)
	job := env.claim(t)

	_, err := env.runner.CompleteJob(bearer(env.token), &factoryv1.CompleteJobRequest{
		JobId: job.Id, Succeeded: true,
		FinishedHarness: factoryv1.Harness_HARNESS_OPENCODE,
		FinishedModel:   "vendor/model",
	})
	require.NoError(t, err)

	got := finishedJob(t, env, task.Id)
	assert.Equal(t, factoryv1.Harness_HARNESS_OPENCODE, got.FinishedHarness)
	assert.Equal(t, "vendor/model", got.FinishedModel)
}

func TestAJobFinishedByItsFirstHarnessNamesNoFallback(t *testing.T) {
	env := browserJobEnv(t)
	task := env.newTask(t, "no fallback")
	env.run(t, task.Id)
	job := env.claim(t)

	_, err := env.runner.CompleteJob(bearer(env.token),
		&factoryv1.CompleteJobRequest{JobId: job.Id, Succeeded: true})
	require.NoError(t, err)

	got := finishedJob(t, env, task.Id)
	assert.Equal(t, factoryv1.Harness_HARNESS_UNSPECIFIED, got.FinishedHarness)
	assert.Empty(t, got.FinishedModel)
}
