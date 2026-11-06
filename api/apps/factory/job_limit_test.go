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

// clearLimits forgets every Claude limit: they are board-wide, so a test that sets
// one must not leave it for the next.
func clearLimits(t *testing.T) {
	t.Helper()
	_, err := testDB.Exec(context.Background(),
		`UPDATE factory.jobs SET limited = false, limit_resets_at = NULL
		WHERE limited`)
	require.NoError(t, err)
}

// limitedBoard starts from a board with no limit and leaves it that way.
func limitedBoard(t *testing.T) jobEnv {
	t.Helper()
	clearLimits(t)
	t.Cleanup(func() { clearLimits(t) })
	return browserJobEnv(t)
}

func claudeLimitedUntil(t *testing.T, env jobEnv) string {
	t.Helper()
	board, err := env.tasks.ListTasks(context.Background(), &factoryv1.ListTasksRequest{})
	require.NoError(t, err)
	return board.ClaudeLimitedUntil
}

func TestAJobTheClaudeLimitStoppedParksItsTaskAndShowsWhenTheLimitLifts(t *testing.T) {
	env := limitedBoard(t)
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
	env := limitedBoard(t)
	task := env.newTask(t, "claims success")
	env.run(t, task.Id, "")
	job := env.claim(t)

	completeLimited(t, env, job.Id, "", true)

	got := env.task(t, task.Id)
	assert.Equal(t, factoryv1.JobStatus_JOB_STATUS_FAILED, got.Jobs[0].Status)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_NEEDS_YOU, got.Task.Status)
	assertHeldForAnHour(t, claudeLimitedUntil(t, env))
}

// assertHeldForAnHour checks the board assumes an hour when no reset time was said.
func assertHeldForAnHour(t *testing.T, until string) {
	t.Helper()
	at, err := time.Parse(time.RFC3339, until)
	require.NoError(t, err, "the limit is still shown: %q", until)
	assert.WithinDuration(t, time.Now().Add(time.Hour), at, 2*time.Minute)
}

func TestAResetTimeThatCannotBeRightIsDropped(t *testing.T) {
	for name, at := range map[string]string{
		"in the past":      time.Now().Add(-time.Hour).UTC().Format(time.RFC3339),
		"months ahead":     time.Now().Add(90 * 24 * time.Hour).UTC().Format(time.RFC3339),
		"not a time":       "tomorrow-ish",
		"a date, no clock": "2030-01-01",
	} {
		env := limitedBoard(t)
		task := env.newTask(t, "odd reset "+name)
		env.run(t, task.Id, "")
		job := env.claim(t)

		completeLimited(t, env, job.Id, at, false)

		got := env.task(t, task.Id)
		assert.True(t, got.Jobs[0].Limited, name)
		assert.Empty(t, got.Jobs[0].LimitResetsAt, name)
		assertHeldForAnHour(t, claudeLimitedUntil(t, env))
	}
}

func TestOnlyTheLatestLimitStillInForceIsShown(t *testing.T) {
	env := limitedBoard(t)
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
	env := limitedBoard(t)
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
	clearLimits(t)
	t.Cleanup(func() { clearLimits(t) })
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

func TestAClaudeLimitInForceHoldsBackAutoPickupOfAClaudeTask(t *testing.T) {
	env := limitedBoard(t)
	first := env.newTask(t, "hits the limit")
	env.run(t, first.Id, "")
	completeLimited(t, env, env.claim(t).Id,
		time.Now().Add(2*time.Hour).UTC().Format(time.RFC3339), false)

	autoPickup(t, true)
	// autoPickup clears limits to start clean; set the limit again for this test.
	second := env.newTask(t, "waits for the limit")
	env.ready(t, second.Id)
	_, err := testDB.Exec(context.Background(), `
		UPDATE factory.jobs SET limited = true,
			limit_resets_at = now() + interval '2 hours'
		WHERE task_id = $1`, first.Id)
	require.NoError(t, err)

	assert.Nil(t, env.claim(t), "a Claude job would only hit the same limit")
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_READY,
		env.task(t, second.Id).Task.Status, "it waits, it is not parked")

	clearLimits(t)
	picked := env.claim(t)
	require.NotNil(t, picked, "once the limit has lifted it is picked up")
	assert.Equal(t, second.Id, picked.TaskId)
}
