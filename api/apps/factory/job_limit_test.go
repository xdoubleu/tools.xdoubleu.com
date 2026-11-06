package factory_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
)

// completeLimited ends a job the way a runner does when the Claude limit stopped it.
func completeLimited(t *testing.T, env jobEnv, jobID, resetsAt string, succeeded bool) {
	t.Helper()
	_, err := env.runner.CompleteJob(bearer(env.token), &factoryv1.CompleteJobRequest{
		JobId: jobID, Succeeded: succeeded, Limited: true, LimitResetsAt: resetsAt,
		Outcome: "Stopped: the Claude usage limit was reached.",
	})
	require.NoError(t, err)
}

func claudeLimitedUntil(t *testing.T, env jobEnv) string {
	t.Helper()
	board, err := env.tasks.ListTasks(context.Background(), &factoryv1.ListTasksRequest{})
	require.NoError(t, err)
	return board.ClaudeLimitedUntil
}

func TestAJobTheClaudeLimitStoppedParksItsTaskAndShowsWhenTheLimitLifts(t *testing.T) {
	env := browserJobEnv(t)
	task := env.newTask(t, "hits the limit")
	env.run(t, task.Id, "")
	job := env.claim(t)
	resets := time.Now().Add(3 * time.Hour).UTC().Truncate(time.Second)

	completeLimited(t, env, job.Id, resets.Format(time.RFC3339), false)

	got := env.task(t, task.Id)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_NEEDS_YOU, got.Task.Status)
	require.Len(t, got.Comments, 1)
	assert.Contains(t, got.Comments[0].Body, "Job stopped by Claude's usage limit")
	assert.Contains(t, got.Comments[0].Body, "usage limit was reached")
	require.Len(t, got.Jobs, 1)
	assert.True(t, got.Jobs[0].Limited)
	assert.Equal(t, resets.Format(time.RFC3339), got.Jobs[0].LimitResetsAt)
	assert.Equal(t, factoryv1.JobStatus_JOB_STATUS_FAILED, got.Jobs[0].Status)

	assert.Equal(t, resets.Format(time.RFC3339), claudeLimitedUntil(t, env),
		"the board header shows when it lifts")
}

func TestALimitedJobNeverCountsAsSucceededWhateverTheRunnerSays(t *testing.T) {
	env := browserJobEnv(t)
	task := env.newTask(t, "claims success")
	env.run(t, task.Id, "")
	job := env.claim(t)

	completeLimited(t, env, job.Id, "", true)

	got := env.task(t, task.Id)
	assert.Equal(t, factoryv1.JobStatus_JOB_STATUS_FAILED, got.Jobs[0].Status)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_NEEDS_YOU, got.Task.Status)
	assert.Empty(t, claudeLimitedUntil(t, env), "no reset time, nothing to show")
}

func TestAResetTimeThatCannotBeRightIsDropped(t *testing.T) {
	for name, at := range map[string]string{
		"in the past":      time.Now().Add(-time.Hour).UTC().Format(time.RFC3339),
		"months ahead":     time.Now().Add(90 * 24 * time.Hour).UTC().Format(time.RFC3339),
		"not a time":       "tomorrow-ish",
		"a date, no clock": "2030-01-01",
	} {
		env := browserJobEnv(t)
		task := env.newTask(t, "odd reset "+name)
		env.run(t, task.Id, "")
		job := env.claim(t)

		completeLimited(t, env, job.Id, at, false)

		got := env.task(t, task.Id)
		assert.True(t, got.Jobs[0].Limited, name)
		assert.Empty(t, got.Jobs[0].LimitResetsAt, name)
		assert.Empty(t, claudeLimitedUntil(t, env), name)
	}
}

func TestOnlyTheLatestLimitStillInForceIsShown(t *testing.T) {
	env := browserJobEnv(t)
	soon := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	later := time.Now().Add(5 * time.Hour).UTC().Truncate(time.Second)
	for _, at := range []time.Time{later, soon} {
		task := env.newTask(t, "limited "+at.Format(time.RFC3339))
		env.run(t, task.Id, "")
		completeLimited(t, env, env.claim(t).Id, at.Format(time.RFC3339), false)
	}

	assert.Equal(t, later.Format(time.RFC3339), claudeLimitedUntil(t, env))
}

func TestAPlainFailureLeavesTheBoardUnlimited(t *testing.T) {
	env := browserJobEnv(t)
	task := env.newTask(t, "just fails")
	env.run(t, task.Id, "")
	_, err := env.runner.CompleteJob(bearer(env.token),
		&factoryv1.CompleteJobRequest{JobId: env.claim(t).Id, Succeeded: false})
	require.NoError(t, err)

	got := env.task(t, task.Id)
	assert.False(t, got.Jobs[0].Limited)
	assert.Empty(t, claudeLimitedUntil(t, env))
}

func TestSlackSaysClaudesLimitStoppedAJobAndUntilWhen(t *testing.T) {
	env, slack := slackEnv(t)
	task := env.newTask(t, "limit in slack")
	env.run(t, task.Id, "")
	job := env.claim(t)
	resets := time.Now().Add(2 * time.Hour).UTC().Truncate(time.Minute)

	completeLimited(t, env, job.Id, resets.Format(time.RFC3339), false)

	assert.Contains(
		t,
		slack.last(),
		":hourglass_flowing_sand: Claude's usage limit stopped a job on",
	)
	assert.Contains(t, slack.last(), "limit in slack")
	assert.Contains(t, slack.last(), "(until "+resets.Format("2006-01-02 15:04")+" UTC)")
}
