package factory_test

import (
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
)

// claimOn claims the next job as a runner whose Mac runs harness and model.
func (e jobEnv) claimOn(
	t *testing.T,
	harness factoryv1.Harness,
	model string,
) (*factoryv1.ClaimedJob, error) {
	t.Helper()
	resp, err := e.runner.ClaimJob(bearer(e.token),
		&factoryv1.ClaimJobRequest{Harness: harness, Model: model})
	if err != nil {
		return nil, err
	}
	return resp.Job, nil
}

func (e jobEnv) jobOf(t *testing.T, taskID string) *factoryv1.Job {
	t.Helper()
	jobs := e.task(t, taskID).Jobs
	require.NotEmpty(t, jobs)
	return jobs[0]
}

func TestAQueuedJobHasNoHarnessUntilARunnerClaimsIt(t *testing.T) {
	env := browserJobEnv(t)
	task := env.newTask(t, "unclaimed")
	job := env.run(t, task.Id, "")
	assert.Equal(t, factoryv1.Harness_HARNESS_UNSPECIFIED, job.Harness)
	assert.Empty(t, job.Model)

	claimed, err := env.claimOn(
		t, factoryv1.Harness_HARNESS_OPENCODE, " vendor/mine:free ",
	)
	require.NoError(t, err)
	require.NotNil(t, claimed)

	ran := env.jobOf(t, task.Id)
	assert.Equal(t, factoryv1.Harness_HARNESS_OPENCODE, ran.Harness)
	assert.Equal(t, "vendor/mine:free", ran.Model)
}

func TestAClaimWithoutAHarnessIsClaudeCode(t *testing.T) {
	env := browserJobEnv(t)
	task := env.newTask(t, "default")
	env.run(t, task.Id, "")
	require.NotNil(t, env.claim(t))

	ran := env.jobOf(t, task.Id)
	assert.Equal(t, factoryv1.Harness_HARNESS_CLAUDE_CODE, ran.Harness)
	assert.Empty(t, ran.Model)
}

func TestAClaimWithABadHarnessOrModelIsRefused(t *testing.T) {
	env := browserJobEnv(t)
	task := env.newTask(t, "bad")
	env.run(t, task.Id, "")

	for name, tc := range map[string]struct {
		harness factoryv1.Harness
		model   string
	}{
		"claude code with a model": {factoryv1.Harness_HARNESS_CLAUDE_CODE, "vendor/model"},
		"opencode without a model": {factoryv1.Harness_HARNESS_OPENCODE, ""},
		"model as an option":       {factoryv1.Harness_HARNESS_OPENCODE, "-x/model"},
		"model without a vendor":   {factoryv1.Harness_HARNESS_OPENCODE, "model"},
		"unknown harness":          {factoryv1.Harness(99), ""},
	} {
		_, err := env.claimOn(t, tc.harness, tc.model)
		assert.Equal(t, connect.CodeInvalidArgument, code(err), name)
	}
	assert.Equal(t, factoryv1.JobStatus_JOB_STATUS_QUEUED, env.jobOf(t, task.Id).Status,
		"the job stays queued")
}

func TestAReplyRecordsTheHarnessOfTheRunnerThatClaimsIt(t *testing.T) {
	env := browserJobEnv(t)
	task := env.newTask(t, "continues")
	env.finishedJob(t, task.Id)

	_, err := env.reply(t, task.Id, "more")
	require.NoError(t, err)
	second, err := env.claimOn(t, factoryv1.Harness_HARNESS_OPENCODE, "vendor/other")
	require.NoError(t, err)
	require.NotNil(t, second)
	assert.Equal(t, task.Id, second.TaskId)
	assert.Equal(t, factoryv1.Harness_HARNESS_OPENCODE, env.jobOf(t, task.Id).Harness)
}
