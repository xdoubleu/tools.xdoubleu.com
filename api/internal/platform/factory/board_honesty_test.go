package factory_test

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
	"tools.xdoubleu.com/internal/platform/githubapp"
)

func taskNumber(t *testing.T, id string) int64 {
	t.Helper()
	n, err := strconv.ParseInt(strings.TrimPrefix(id, "T-"), 10, 64)
	require.NoError(t, err)
	return n
}

// setStatus puts a task in a column, last touched two hours ago.
func setStatus(t *testing.T, id, status string) {
	t.Helper()
	_, err := testDB.Exec(context.Background(), `
		UPDATE factory.tasks SET status = $2, updated_at = now() - interval '2 hours'
		WHERE number = $1`, taskNumber(t, id), status)
	require.NoError(t, err)
}

// linkPR records a pull request for the task in the given state.
func linkPR(t *testing.T, id, state string) {
	t.Helper()
	_, err := testDB.Exec(context.Background(), `
		INSERT INTO factory.linked_pull_requests (url, task_id, state)
		SELECT 'https://github.com/xdoubleu/tools.xdoubleu.com/pull/'
			|| number, id, $2 FROM factory.tasks WHERE number = $1`,
		taskNumber(t, id), state)
	require.NoError(t, err)
}

func (e jobEnv) lose(t *testing.T, jobID string) {
	t.Helper()
	_, err := testDB.Exec(context.Background(), `
		UPDATE factory.jobs SET last_check_in = now() - interval '6 minutes'
		WHERE id = $1`, jobID)
	require.NoError(t, err)
}

func (e jobEnv) lostTask(t *testing.T, state string) string {
	t.Helper()
	task := e.newTask(t, "lost "+uuid.NewString())
	e.run(t, task.Id)
	claimed := e.claim(t)
	if state != "" {
		linkPR(t, task.Id, state)
	}
	e.lose(t, claimed.Id)
	return task.Id
}

func lastComment(r *factoryv1.GetTaskResponse) string {
	return r.Comments[len(r.Comments)-1].Body
}

func TestALostJobWhoseTaskMergedGoesToTestingNotNeedsYou(t *testing.T) {
	env := browserJobEnv(t)
	id := env.lostTask(t, "merged")

	_, err := env.tasks.ListTasks(context.Background(), &factoryv1.ListTasksRequest{})
	require.NoError(t, err)
	got := env.task(t, id)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_TESTING, got.Task.Status)
	assert.Contains(t, lastComment(got), "merged")
}

func TestALostJobWithAnOpenPullRequestGoesToInReview(t *testing.T) {
	env := browserJobEnv(t)
	id := env.lostTask(t, "open")

	got := env.task(t, id)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_IN_REVIEW, got.Task.Status)
	assert.Contains(t, lastComment(got), "open")
}

func TestALostJobWithoutAPullRequestIsQueuedAgainUntilTheCap(t *testing.T) {
	env := browserJobEnv(t)
	id := env.lostTask(t, "")

	got := env.task(t, id)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_IN_PROGRESS, got.Task.Status)
	assert.Contains(t, lastComment(got), "Job lost")
	assert.Contains(t, lastComment(got), "Queued again")

	for range 2 {
		again := env.claim(t)
		require.NotNil(t, again)
		assert.Equal(t, id, again.TaskId)
		env.lose(t, again.Id)
		_, err := env.tasks.ListTasks(context.Background(), &factoryv1.ListTasksRequest{})
		require.NoError(t, err)
	}
	// Two requeues are spent: nothing is left to resume.
	got = env.task(t, id)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_NEEDS_YOU, got.Task.Status)
	assert.Nil(t, env.claim(t))
}

func TestAParkedTaskIsInBacklogNotInProgress(t *testing.T) {
	env, _ := parkEnv(t)
	id := env.blockedJob(t, "parked "+uuid.NewString())

	got := env.task(t, id)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_BACKLOG, got.Task.Status)
	assert.Contains(t, got.Task.Labels, "waiting-on-sandbox")
	assert.Contains(t, lastComment(got), "Parked")
}

func TestATaskIsNeverParkedWaitingOnItself(t *testing.T) {
	env, _ := parkEnv(t)
	problem := "no jq " + uuid.NewString()
	created, err := env.tasks.CreateTask(context.Background(),
		&factoryv1.CreateTaskRequest{
			Title: "Sandbox: " + problem, Body: "Fix it.", RepoId: seededRepoID(t),
			Labels: []string{"factory-sandbox", "bug"},
		})
	require.NoError(t, err)
	env.run(t, created.Task.Id)
	claimed := env.claim(t)
	_, err = env.runner.CompleteJob(bearer(env.token), &factoryv1.CompleteJobRequest{
		JobId: claimed.Id, SessionId: "s", Blocked: problem,
	})
	require.NoError(t, err)

	got := env.task(t, created.Task.Id)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_NEEDS_YOU, got.Task.Status)
	assert.NotContains(t, got.Task.Labels, "waiting-on-sandbox")
}

func TestAGapNoImageChangeFixesIsABugTaskNotAParkedTask(t *testing.T) {
	env, _ := parkEnv(t)
	gap := "the host has no network " + uuid.NewString()
	blocked := gap + ": no Containerfile fix applies"
	for range 2 {
		task := env.newTask(t, "work "+uuid.NewString())
		env.run(t, task.Id)
		claimed := env.claim(t)
		_, err := env.runner.CompleteJob(bearer(env.token), &factoryv1.CompleteJobRequest{
			JobId: claimed.Id, SessionId: "s", Blocked: blocked,
		})
		require.NoError(t, err)
		got := env.task(t, task.Id)
		assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_NEEDS_YOU, got.Task.Status)
		assert.NotContains(t, got.Task.Labels, "waiting-on-sandbox")
	}

	list, err := env.tasks.ListTasks(context.Background(), &factoryv1.ListTasksRequest{})
	require.NoError(t, err)
	var bugs []*factoryv1.Task
	for _, task := range list.Tasks {
		if task.Title == gap+": no Containerfile fix applies" {
			bugs = append(bugs, task)
		}
	}
	require.Len(t, bugs, 1, "the second report reuses the open task")
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_BACKLOG, bugs[0].Status)
	assert.NotContains(t, bugs[0].Labels, "factory-sandbox")
}

func TestARoutineBlockedEveryRunFilesOneSandboxTask(t *testing.T) {
	env := routineEnv(t)
	env.saveRoutine(t, "noop", false, 0)
	for i := range 3 {
		_, err := env.tasks.RunRoutine(context.Background(),
			&factoryv1.RunRoutineRequest{Name: "noop"})
		require.NoError(t, err)
		claimed := env.claim(t)
		_, err = env.runner.CompleteJob(bearer(env.token), &factoryv1.CompleteJobRequest{
			JobId: claimed.Id, Blocked: "no gh token on run " + strconv.Itoa(i) +
				" " + uuid.NewString(),
		})
		require.NoError(t, err)
	}
	found := sandboxTasks(t, env, "Sandbox: routine noop is blocked")
	require.Len(t, found, 1)
	assert.Len(t, withoutPickupNotes(env.task(t, found[0].Id).Comments), 3)
}

func TestTheReconcilerMovesInProgressTasksWithoutAJob(t *testing.T) {
	env := browserJobEnv(t)

	parked := env.newTask(t, "parked "+uuid.NewString())
	setStatus(t, parked.Id, "in_progress")
	_, err := testDB.Exec(context.Background(),
		`UPDATE factory.tasks SET labels = '{waiting-on-sandbox}' WHERE number = $1`,
		taskNumber(t, parked.Id))
	require.NoError(t, err)

	merged := env.newTask(t, "merged "+uuid.NewString())
	setStatus(t, merged.Id, "in_progress")
	linkPR(t, merged.Id, "merged")

	open := env.newTask(t, "open "+uuid.NewString())
	setStatus(t, open.Id, "in_progress")
	linkPR(t, open.Id, "open")

	nothing := env.newTask(t, "nothing "+uuid.NewString())
	setStatus(t, nothing.Id, "in_progress")

	fresh := env.newTask(t, "fresh "+uuid.NewString())
	_, err = testDB.Exec(context.Background(),
		`UPDATE factory.tasks SET status = 'in_progress' WHERE number = $1`,
		taskNumber(t, fresh.Id))
	require.NoError(t, err)

	running := env.newTask(t, "running "+uuid.NewString())
	env.run(t, running.Id)
	require.NotNil(t, env.claim(t))
	setStatus(t, running.Id, "in_progress")

	require.NoError(t, env.app.ReconcileBoardNow(context.Background()))

	status := func(id string) factoryv1.TaskStatus { return env.task(t, id).Task.Status }
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_BACKLOG, status(parked.Id))
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_TESTING, status(merged.Id))
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_IN_REVIEW, status(open.Id))
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_NEEDS_YOU, status(nothing.Id))
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_IN_PROGRESS, status(fresh.Id),
		"a task touched recently may be someone's own session")
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_IN_PROGRESS, status(running.Id))
	assert.Contains(t, lastComment(env.task(t, merged.Id)), "merged")
}

func TestTheReconcilerMovesNeedsYouTasksWhosePullRequestMerged(t *testing.T) {
	env := browserJobEnv(t)

	merged := env.newTask(t, "merged "+uuid.NewString())
	setStatus(t, merged.Id, "needs_you")
	linkPR(t, merged.Id, "merged")

	open := env.newTask(t, "open "+uuid.NewString())
	setStatus(t, open.Id, "needs_you")
	linkPR(t, open.Id, "open")

	require.NoError(t, env.app.ReconcileBoardNow(context.Background()))
	status := func(id string) factoryv1.TaskStatus { return env.task(t, id).Task.Status }
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_TESTING, status(merged.Id))
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_NEEDS_YOU, status(open.Id))
}

func (e jobEnv) lostWithPull(t *testing.T) string {
	t.Helper()
	task := e.newTask(t, "polled late "+uuid.NewString())
	e.run(t, task.Id)
	claimed := e.claim(t)
	_, err := testDB.Exec(context.Background(), `
		UPDATE factory.jobs SET pr_url = $2, pr_state = 'open' WHERE id = $1`,
		claimed.Id, ownPull)
	require.NoError(t, err)
	e.lose(t, claimed.Id)
	return task.Id
}

func TestALostJobReadsItsPullRequestFromGitHubNotTheStoredState(t *testing.T) {
	env, gh := githubEnv(t)
	status := ended(githubapp.StateMerged)
	status.MergeSHA = "abc123"
	gh.pulls[42] = pullAnswer{status: status, err: nil}
	id := env.lostWithPull(t)

	got := env.task(t, id)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_TESTING, got.Task.Status)
	var sha string
	require.NoError(t, testDB.QueryRow(context.Background(),
		`SELECT merge_sha FROM factory.tasks WHERE number = $1`,
		taskNumber(t, id)).Scan(&sha))
	assert.Equal(t, "abc123", sha)
}

func TestALostJobWhoseGitHubReadFailsFallsBackToTheStoredState(t *testing.T) {
	env, gh := githubEnv(t)
	gh.pulls[42] = pullAnswer{status: ended(""), err: errors.New("github is down")}
	id := env.lostWithPull(t)

	got := env.task(t, id)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_IN_REVIEW, got.Task.Status)
}
