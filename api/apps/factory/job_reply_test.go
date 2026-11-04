package factory_test

import (
	"context"
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
	"tools.xdoubleu.com/gen/factory/v1/factoryv1connect"
	"tools.xdoubleu.com/internal/constants"
)

func (e jobEnv) reply(t *testing.T, taskID, text string) (*factoryv1.Job, error) {
	t.Helper()
	resp, err := e.tasks.ReplyTask(context.Background(),
		&factoryv1.ReplyTaskRequest{TaskId: taskID, Text: text})
	if err != nil {
		return nil, err
	}
	return resp.Job, nil
}

// finishedJob runs a task's job to the end, with the harness session "sess-1",
// and returns the job as claimed.
func (e jobEnv) finishedJob(t *testing.T, taskID string) *factoryv1.ClaimedJob {
	t.Helper()
	e.run(t, taskID, "")
	claimed := e.claim(t)
	require.NotNil(t, claimed)
	_, err := e.runner.CheckIn(bearer(e.token),
		&factoryv1.CheckInRequest{JobId: claimed.Id, SessionId: "sess-1"})
	require.NoError(t, err)
	_, err = e.runner.CompleteJob(bearer(e.token),
		&factoryv1.CompleteJobRequest{JobId: claimed.Id, Succeeded: true})
	require.NoError(t, err)
	return claimed
}

func TestReplyResumesTheLastJobsSessionOnItsBranch(t *testing.T) {
	env := browserJobEnv(t)
	task := env.newTask(t, "fix login")
	first := env.finishedJob(t, task.Id)
	assert.Empty(t, first.ResumeSessionId)

	job, err := env.reply(t, task.Id, "  CI is red, fix it  ")
	require.NoError(t, err)
	assert.Equal(t, factoryv1.JobStatus_JOB_STATUS_QUEUED, job.Status)
	assert.Equal(t, first.Branch, job.Branch)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_IN_PROGRESS,
		env.task(t, task.Id).Task.Status)

	second := env.claim(t)
	require.NotNil(t, second)
	assert.Equal(t, job.Id, second.Id)
	assert.Equal(t, "CI is red, fix it", second.Prompt)
	assert.Equal(t, first.Branch, second.Branch)
	assert.Equal(t, first.Harness, second.Harness)
	assert.Equal(t, "sess-1", second.ResumeSessionId)
}

func TestAReplyToAReplyResumesTheLatestSession(t *testing.T) {
	env := browserJobEnv(t)
	task := env.newTask(t, "twice")
	env.finishedJob(t, task.Id)

	_, err := env.reply(t, task.Id, "one more thing")
	require.NoError(t, err)
	second := env.claim(t)
	_, err = env.runner.CheckIn(bearer(env.token),
		&factoryv1.CheckInRequest{JobId: second.Id, SessionId: "sess-2"})
	require.NoError(t, err)
	_, err = env.runner.CompleteJob(bearer(env.token),
		&factoryv1.CompleteJobRequest{JobId: second.Id, Succeeded: true})
	require.NoError(t, err)

	_, err = env.reply(t, task.Id, "and another")
	require.NoError(t, err)
	assert.Equal(t, "sess-2", env.claim(t).ResumeSessionId)
}

func TestReplyIsRefusedWithoutAFinishedJobWithASession(t *testing.T) {
	env := browserJobEnv(t)
	task := env.newTask(t, "refused")

	_, err := env.reply(t, task.Id, "hello")
	assert.Equal(t, connect.CodeAlreadyExists, code(err), "no job yet")

	env.run(t, task.Id, "")
	_, err = env.reply(t, task.Id, "hello")
	assert.Equal(t, connect.CodeAlreadyExists, code(err), "job still queued")

	claimed := env.claim(t)
	_, err = env.runner.CompleteJob(bearer(env.token),
		&factoryv1.CompleteJobRequest{JobId: claimed.Id, Succeeded: false})
	require.NoError(t, err)
	_, err = env.reply(t, task.Id, "hello")
	assert.Equal(t, connect.CodeAlreadyExists, code(err), "no session to resume")
}

func TestReplyNeedsTextAndAKnownTask(t *testing.T) {
	env := browserJobEnv(t)
	task := env.newTask(t, "text")
	env.finishedJob(t, task.Id)

	for name, text := range map[string]string{
		"empty":    "",
		"blank":    "  \n ",
		"too long": string(make([]rune, 20001)),
	} {
		_, err := env.reply(t, task.Id, text)
		assert.Equal(t, connect.CodeInvalidArgument, code(err), name)
	}
	_, err := env.reply(t, "not-a-uuid", "hello")
	assert.Equal(t, connect.CodeInvalidArgument, code(err))
	_, err = env.reply(t, uuid.NewString(), "hello")
	assert.Equal(t, connect.CodeNotFound, code(err))
}

func TestReplyNeedsAHumanAdminInABrowser(t *testing.T) {
	env := browserJobEnv(t)
	task := env.newTask(t, "approval")
	env.finishedJob(t, task.Id)

	for name, other := range map[string]factoryv1connect.TaskServiceClient{
		"not an admin":  newClientKeepingBoard(t, false, constants.ChannelBrowser),
		"not a browser": newClientKeepingBoard(t, true, constants.ChannelMCP),
	} {
		_, err := other.ReplyTask(context.Background(),
			&factoryv1.ReplyTaskRequest{TaskId: task.Id, Text: "hello"})
		assert.Equal(t, connect.CodePermissionDenied, code(err), name)
	}
}

func TestReplyIsRefusedOnADoneTask(t *testing.T) {
	env := browserJobEnv(t)
	task := env.newTask(t, "done")
	env.finishedJob(t, task.Id)
	_, err := env.tasks.MoveTask(context.Background(), &factoryv1.MoveTaskRequest{
		Id: task.Id, Status: factoryv1.TaskStatus_TASK_STATUS_DONE,
	})
	require.NoError(t, err)

	_, err = env.reply(t, task.Id, "hello")
	assert.Equal(t, connect.CodeAlreadyExists, code(err))
}
