package factory_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
)

// waitingJob runs a task to a completion that waits on dep, with a session when
// session is true.
func (e jobEnv) waitingJob(t *testing.T, dep string, session bool) string {
	t.Helper()
	task := e.newTask(t, "waits "+uuid.NewString())
	e.run(t, task.Id)
	claimed := e.claim(t)
	sessionID := ""
	if session {
		sessionID = "sess-" + claimed.Id
	}
	_, err := e.runner.CompleteJob(bearer(e.token), &factoryv1.CompleteJobRequest{
		JobId: claimed.Id, SessionId: sessionID, WaitingOn: []string{dep},
	})
	require.NoError(t, err)
	return task.Id
}

func TestAWaitingTaskResumesItsSessionOnceItsDependencyIsDone(t *testing.T) {
	env, _ := parkEnv(t)
	dep := env.newTask(t, "dependency "+uuid.NewString())
	id := env.waitingJob(t, dep.Id, true)
	assert.Contains(t, lastComment(env.task(t, id)), "resumes by itself")

	require.NoError(t, env.app.ResumeParkedNow(context.Background()))
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_BACKLOG, env.task(t, id).Task.Status)
	assert.Nil(t, env.claim(t))

	setStatus(t, dep.Id, "done")
	require.NoError(t, env.app.ResumeParkedNow(context.Background()))
	got := env.task(t, id)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_IN_PROGRESS, got.Task.Status)
	assert.Contains(t, lastComment(got), "resumes on its branch")
	resumed := env.claim(t)
	require.NotNil(t, resumed)
	assert.Equal(t, id, resumed.TaskId)
	assert.Contains(t, resumed.Prompt, "you were waiting on are merged")
	assert.NotEmpty(t, resumed.ResumeSessionId)

	// The wait is spent: a later run resumes nothing more.
	require.NoError(t, env.app.ResumeParkedNow(context.Background()))
	assert.Nil(t, env.claim(t))
}

func TestAWaitingTaskWithoutASessionGoesToNeedsYouOnceItsDepMerges(t *testing.T) {
	env, _ := parkEnv(t)
	dep := env.newTask(t, "dependency "+uuid.NewString())
	id := env.waitingJob(t, dep.Id, false)

	_, err := testDB.Exec(context.Background(),
		`UPDATE factory.tasks SET merge_sha = 'abc' WHERE number = $1`,
		taskNumber(t, dep.Id))
	require.NoError(t, err)
	require.NoError(t, env.app.ResumeParkedNow(context.Background()))

	got := env.task(t, id)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_NEEDS_YOU, got.Task.Status)
	assert.Contains(t, lastComment(got), "no session to resume")
	assert.Nil(t, env.claim(t))
}

func TestMovingAWaitingTaskEndsItsWait(t *testing.T) {
	env, _ := parkEnv(t)
	dep := env.newTask(t, "dependency "+uuid.NewString())
	id := env.waitingJob(t, dep.Id, true)

	env.moveTo(t, id, factoryv1.TaskStatus_TASK_STATUS_NEEDS_YOU)
	env.moveTo(t, id, factoryv1.TaskStatus_TASK_STATUS_BACKLOG)
	setStatus(t, dep.Id, "done")
	require.NoError(t, env.app.ResumeParkedNow(context.Background()))

	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_BACKLOG, env.task(t, id).Task.Status)
	assert.Nil(t, env.claim(t))
}

func TestASplitTaskIsDoneOnceItsSlicesMerge(t *testing.T) {
	env, _ := parkEnv(t)
	task := env.newTask(t, "too big "+uuid.NewString())
	env.run(t, task.Id)
	claimed := env.claim(t)
	slice := env.newLabelledTask(t, "slice "+uuid.NewString(), "epic:big")
	_, err := env.runner.CompleteJob(bearer(env.token), &factoryv1.CompleteJobRequest{
		JobId: claimed.Id, SessionId: "sess", WaitingOn: []string{slice.Id},
	})
	require.NoError(t, err)

	require.NoError(t, env.app.ResumeParkedNow(context.Background()))
	assert.Equal(
		t,
		factoryv1.TaskStatus_TASK_STATUS_BACKLOG,
		env.task(t, task.Id).Task.Status,
	)

	setStatus(t, slice.Id, "done")
	require.NoError(t, env.app.ResumeParkedNow(context.Background()))
	got := env.task(t, task.Id)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_DONE, got.Task.Status)
	assert.Contains(t, lastComment(got), "Split into")
	assert.Nil(t, env.claim(t))
}

func TestADependentOfASplitTaskIsPickedUpOnceTheSplitResolves(t *testing.T) {
	env, _ := parkEnv(t)
	autoPickup(t, true)
	task := env.newTask(t, "too big "+uuid.NewString())
	env.run(t, task.Id)
	claimed := env.claim(t)
	slice := env.newLabelledTask(t, "slice "+uuid.NewString(), "epic:big")
	_, err := env.runner.CompleteJob(bearer(env.token), &factoryv1.CompleteJobRequest{
		JobId: claimed.Id, SessionId: "sess", WaitingOn: []string{slice.Id},
	})
	require.NoError(t, err)
	dependent := seedTask(
		t,
		"dependent",
		"Depends on: "+task.Id+"\n\nDo it.",
		"ready",
		nil,
	)

	assert.Nil(t, env.claim(t))
	env.assertSkipped(t, dependent, "depends on "+task.Id)

	setStatus(t, slice.Id, "done")
	require.NoError(t, env.app.ResumeParkedNow(context.Background()))
	picked := env.claim(t)
	require.NotNil(t, picked)
	assert.Equal(t, dependent.ref(), picked.TaskId)
}

func TestAJobWaitingOnlyOnMergedTasksGoesToNeedsYouNotAResume(t *testing.T) {
	env, _ := parkEnv(t)
	dep := env.newTask(t, "dependency "+uuid.NewString())
	setStatus(t, dep.Id, "done")
	task := env.newTask(t, "waits "+uuid.NewString())
	env.run(t, task.Id)
	claimed := env.claim(t)
	_, err := env.runner.CompleteJob(bearer(env.token), &factoryv1.CompleteJobRequest{
		JobId: claimed.Id, SessionId: "sess-" + claimed.Id,
		Outcome: "the real blocker is elsewhere", WaitingOn: []string{dep.Id},
	})
	require.NoError(t, err)

	got := env.task(t, task.Id)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_NEEDS_YOU, got.Task.Status)
	assert.Contains(t, lastComment(got), "already merged")
	assert.Contains(t, lastComment(got), "the real blocker is elsewhere")
	require.NoError(t, env.app.ResumeParkedNow(context.Background()))
	assert.Nil(t, env.claim(t), "nothing resumes it")
}
