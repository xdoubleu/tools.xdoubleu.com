package factory_test

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/factory"
	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
	"tools.xdoubleu.com/gen/factory/v1/factoryv1connect"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/testhelper"
)

// jobEnv is a paired runner and an admin in a browser, on one app.
type jobEnv struct {
	tasks  factoryv1connect.TaskServiceClient
	runner factoryv1connect.RunnerServiceClient
	token  string
}

func newJobEnv(t *testing.T, asAdmin bool, channel string) jobEnv {
	t.Helper()
	cleanRunners(t)
	_, err := testDB.Exec(context.Background(), "DELETE FROM factory.jobs")
	require.NoError(t, err)

	app := newAppWith(asAdmin, channel,
		factory.WithStepUp(stepUpReturning(nil)),
		factory.WithHeartbeatHold(20*time.Millisecond),
	)
	ts := httptest.NewServer(testhelper.BuildMux(app))
	t.Cleanup(ts.Close)
	transport := connecthttp.NewTransport(
		ts.Client(), ts.URL, connecthttp.WithReadMaxBytes(0),
	)
	env := runnerEnv{
		admin:  factoryv1connect.NewRunnerAdminServiceClient(connect.NewClient(transport)),
		runner: factoryv1connect.NewRunnerServiceClient(connect.NewClient(transport)),
	}
	created := createRunner(t, env, "Home Mac")
	return jobEnv{
		tasks:  factoryv1connect.NewTaskServiceClient(connect.NewClient(transport)),
		runner: env.runner,
		token:  pair(t, env, created.PairingCode).DeviceToken,
	}
}

func browserJobEnv(t *testing.T) jobEnv {
	t.Helper()
	return newJobEnv(t, true, constants.ChannelBrowser)
}

func (e jobEnv) newTask(t *testing.T, title string) *factoryv1.Task {
	t.Helper()
	resp, err := e.tasks.CreateTask(context.Background(), &factoryv1.CreateTaskRequest{
		Title: title, Body: "Do the thing.", RepoId: seededRepoID(t),
	})
	require.NoError(t, err)
	return resp.Task
}

func (e jobEnv) run(t *testing.T, taskID, text string) *factoryv1.Job {
	t.Helper()
	resp, err := e.tasks.RunTask(context.Background(),
		&factoryv1.RunTaskRequest{TaskId: taskID, Text: text})
	require.NoError(t, err)
	return resp.Job
}

func (e jobEnv) claim(t *testing.T) *factoryv1.ClaimedJob {
	t.Helper()
	resp, err := e.runner.ClaimJob(bearer(e.token), &factoryv1.ClaimJobRequest{})
	require.NoError(t, err)
	return resp.Job
}

func (e jobEnv) task(t *testing.T, id string) *factoryv1.GetTaskResponse {
	t.Helper()
	resp, err := e.tasks.GetTask(context.Background(), &factoryv1.GetTaskRequest{Id: id})
	require.NoError(t, err)
	return resp
}

func TestRunQueuesAJobFreezesThePromptAndStartsTheTask(t *testing.T) {
	env := browserJobEnv(t)
	task := env.newTask(t, "Fix the Login Page!")

	job := env.run(t, task.Id, "  Mind the tests.  ")
	assert.Equal(t, factoryv1.JobStatus_JOB_STATUS_QUEUED, job.Status)
	assert.Equal(t, task.Id, job.TaskId)
	assert.Equal(t, "agent/"+task.Id+"-fix-the-login-page", job.Branch)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_IN_PROGRESS,
		env.task(t, task.Id).Task.Status)

	claimed := env.claim(t)
	require.NotNil(t, claimed)
	assert.Contains(t, claimed.Prompt, "# Task: Fix the Login Page!")
	assert.Contains(t, claimed.Prompt, "Do the thing.")
	assert.True(t, strings.HasSuffix(claimed.Prompt, "Mind the tests."))
	assert.Equal(t, "xdoubleu", claimed.RepoOwner)
	assert.Equal(t, "tools.xdoubleu.com", claimed.RepoName)
	assert.Equal(t, job.Branch, claimed.Branch)
	assert.Equal(t, job.Id, claimed.Id)

	// Edits after Run don't reach a job that already has its prompt.
	_, err := env.tasks.UpdateTask(context.Background(), &factoryv1.UpdateTaskRequest{
		Id: task.Id, Title: "changed", Body: "other",
	})
	require.NoError(t, err)
	assert.Contains(t, claimed.Prompt, "Do the thing.")
}

func TestRunNeedsAHumanAdminInABrowser(t *testing.T) {
	env := browserJobEnv(t)
	task := env.newTask(t, "needs approval")

	for name, other := range map[string]factoryv1connect.TaskServiceClient{
		"not an admin":  newClientKeepingBoard(t, false, constants.ChannelBrowser),
		"not a browser": newClientKeepingBoard(t, true, constants.ChannelMCP),
	} {
		_, err := other.RunTask(context.Background(),
			&factoryv1.RunTaskRequest{TaskId: task.Id})
		assert.Equal(t, connect.CodePermissionDenied, code(err), name)
	}
}

func TestRunRefusesABodyEditedOutsideABrowser(t *testing.T) {
	// The task is created through MCP, so its body was never seen in a browser.
	agent := newClientKeepingBoard(t, true, constants.ChannelMCP)
	created, err := agent.CreateTask(context.Background(), &factoryv1.CreateTaskRequest{
		Title: "from an agent", Body: "Do the thing.", RepoId: seededRepoID(t),
	})
	require.NoError(t, err)
	task := created.Task

	browser := browserJobEnv(t)
	_, err = browser.tasks.RunTask(context.Background(),
		&factoryv1.RunTaskRequest{TaskId: task.Id})
	assert.Equal(t, connect.CodeInvalidArgument, code(err))

	got := browser.task(t, task.Id)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_NEEDS_YOU, got.Task.Status)
	require.Len(t, got.Comments, 1)
	assert.Equal(t, "system:factory", got.Comments[0].Author)
	assert.Contains(t, got.Comments[0].Body, "outside a browser")
	assert.Nil(t, browser.claim(t), "no job was queued")
}

func TestRunRefusesASecondJobForATask(t *testing.T) {
	env := browserJobEnv(t)
	task := env.newTask(t, "once")
	env.run(t, task.Id, "")
	_, err := env.tasks.RunTask(context.Background(),
		&factoryv1.RunTaskRequest{TaskId: task.Id})
	assert.Equal(t, connect.CodeAlreadyExists, code(err))

	_, err = env.tasks.RunTask(context.Background(),
		&factoryv1.RunTaskRequest{TaskId: "nope"})
	assert.Equal(t, connect.CodeInvalidArgument, code(err))
	_, err = env.tasks.RunTask(context.Background(), &factoryv1.RunTaskRequest{
		TaskId: task.Id, Text: strings.Repeat("x", 10001),
	})
	assert.Equal(t, connect.CodeInvalidArgument, code(err))
}

func TestJobsRunOldestFirstAndOneAtATime(t *testing.T) {
	env := browserJobEnv(t)
	first := env.newTask(t, "first")
	second := env.newTask(t, "second")
	env.run(t, first.Id, "")
	time.Sleep(5 * time.Millisecond)
	env.run(t, second.Id, "")

	one := env.claim(t)
	require.NotNil(t, one)
	assert.Equal(t, first.Id, one.TaskId, "FIFO")
	assert.Nil(t, env.claim(t), "one at a time: the second waits")

	_, err := env.runner.CompleteJob(bearer(env.token),
		&factoryv1.CompleteJobRequest{JobId: one.Id, Succeeded: true})
	require.NoError(t, err)
	two := env.claim(t)
	require.NotNil(t, two)
	assert.Equal(t, second.Id, two.TaskId)
}

func TestCheckInKeepsTheSessionAndCompleteCommentsTheOutcome(t *testing.T) {
	env := browserJobEnv(t)
	task := env.newTask(t, "finishes")
	env.run(t, task.Id, "")
	claimed := env.claim(t)

	resp, err := env.runner.CheckIn(bearer(env.token),
		&factoryv1.CheckInRequest{JobId: claimed.Id, SessionId: "sess-1"})
	require.NoError(t, err)
	assert.False(t, resp.Stop)

	_, err = env.runner.CompleteJob(bearer(env.token), &factoryv1.CompleteJobRequest{
		JobId: claimed.Id, Succeeded: true, Outcome: "  Added the thing.  ",
	})
	require.NoError(t, err)

	var session, status string
	require.NoError(t, testDB.QueryRow(context.Background(),
		`SELECT session_id, status FROM factory.jobs WHERE id = $1`, claimed.Id,
	).Scan(&session, &status))
	assert.Equal(t, "sess-1", session)
	assert.Equal(t, "succeeded", status)

	got := env.task(t, task.Id)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_NEEDS_YOU, got.Task.Status)
	require.Len(t, got.Comments, 1)
	assert.Contains(t, got.Comments[0].Author, "runner:")
	assert.Contains(t, got.Comments[0].Body, "succeeded")
	assert.Contains(t, got.Comments[0].Body, "Added the thing.")

	// A job can finish once.
	_, err = env.runner.CompleteJob(bearer(env.token),
		&factoryv1.CompleteJobRequest{JobId: claimed.Id})
	assert.Equal(t, connect.CodeAlreadyExists, code(err))
}

func TestAFailedJobAndAnEmptyOutcome(t *testing.T) {
	env := browserJobEnv(t)
	task := env.newTask(t, "fails")
	env.run(t, task.Id, "")
	claimed := env.claim(t)
	_, err := env.runner.CompleteJob(bearer(env.token),
		&factoryv1.CompleteJobRequest{JobId: claimed.Id, Succeeded: false})
	require.NoError(t, err)

	got := env.task(t, task.Id)
	require.Len(t, got.Comments, 1)
	assert.Contains(t, got.Comments[0].Body, "failed")
	assert.Contains(t, got.Comments[0].Body, "wrote no outcome")
}

func TestAJobThatStopsCheckingInIsLost(t *testing.T) {
	env := browserJobEnv(t)
	task := env.newTask(t, "vanishes")
	env.run(t, task.Id, "")
	claimed := env.claim(t)

	_, err := testDB.Exec(context.Background(),
		`UPDATE factory.jobs SET last_check_in = now() - interval '6 minutes'
		 WHERE id = $1`, claimed.Id)
	require.NoError(t, err)

	// Noticed when the board is read, though the runner is gone.
	_, err = env.tasks.ListTasks(context.Background(), &factoryv1.ListTasksRequest{})
	require.NoError(t, err)

	got := env.task(t, task.Id)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_NEEDS_YOU, got.Task.Status)
	require.Len(t, got.Comments, 1)
	assert.Equal(t, "system:factory", got.Comments[0].Author)
	assert.Contains(t, got.Comments[0].Body, "Job lost")

	resp, err := env.runner.CheckIn(bearer(env.token),
		&factoryv1.CheckInRequest{JobId: claimed.Id})
	require.NoError(t, err)
	assert.True(t, resp.Stop, "a lost job tells the runner to stop")

	_, err = env.runner.CompleteJob(bearer(env.token),
		&factoryv1.CompleteJobRequest{JobId: claimed.Id, Succeeded: true})
	assert.Equal(t, connect.CodeAlreadyExists, code(err), "too late to complete")
}

func TestAJobStillCheckingInIsNotLost(t *testing.T) {
	env := browserJobEnv(t)
	task := env.newTask(t, "alive")
	env.run(t, task.Id, "")
	claimed := env.claim(t)
	_, err := testDB.Exec(context.Background(),
		`UPDATE factory.jobs SET last_check_in = now() - interval '4 minutes'
		 WHERE id = $1`, claimed.Id)
	require.NoError(t, err)

	_, err = env.tasks.ListTasks(context.Background(), &factoryv1.ListTasksRequest{})
	require.NoError(t, err)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_IN_PROGRESS,
		env.task(t, task.Id).Task.Status)
}

func TestClaimJobHoldsThenAnswersEmpty(t *testing.T) {
	env := browserJobEnv(t)
	start := time.Now()
	assert.Nil(t, env.claim(t))
	assert.GreaterOrEqual(t, time.Since(start), 15*time.Millisecond, "held for the hold")
}

func TestTheRunnerCallsNeedADeviceToken(t *testing.T) {
	env := browserJobEnv(t)
	for name, call := range map[string]func(context.Context) error{
		"claim": func(ctx context.Context) error {
			_, err := env.runner.ClaimJob(ctx, &factoryv1.ClaimJobRequest{})
			return err
		},
		"check in": func(ctx context.Context) error {
			_, err := env.runner.CheckIn(ctx, &factoryv1.CheckInRequest{})
			return err
		},
		"complete": func(ctx context.Context) error {
			_, err := env.runner.CompleteJob(ctx, &factoryv1.CompleteJobRequest{})
			return err
		},
	} {
		assert.Equal(t, connect.CodeUnauthenticated, code(call(context.Background())), name)
		assert.Equal(t, connect.CodeUnauthenticated, code(call(bearer("frt_unknown"))), name)
	}

	for name, call := range map[string]func() error{
		"check in": func() error {
			_, err := env.runner.CheckIn(bearer(env.token),
				&factoryv1.CheckInRequest{JobId: "x"})
			return err
		},
		"complete": func() error {
			_, err := env.runner.CompleteJob(bearer(env.token),
				&factoryv1.CompleteJobRequest{JobId: "x"})
			return err
		},
	} {
		assert.Equal(t, connect.CodeInvalidArgument, code(call()), name+": bad id")
	}
}

func TestTheDatabaseKeepsOneUnfinishedJobPerTask(t *testing.T) {
	env := browserJobEnv(t)
	task := env.newTask(t, "raced")
	env.run(t, task.Id, "")

	_, err := testDB.Exec(context.Background(), `
		INSERT INTO factory.jobs (task_id, repo_id, harness, prompt, branch, created_by)
		SELECT task_id, repo_id, harness, prompt, branch, created_by
		FROM factory.jobs WHERE task_id = $1`, task.Id)
	require.Error(t, err, "a second queued job for the task is refused")
}

func TestARepeatedRefusalCommentsOnce(t *testing.T) {
	agent := newClientKeepingBoard(t, true, constants.ChannelMCP)
	created, err := agent.CreateTask(context.Background(), &factoryv1.CreateTaskRequest{
		Title: "from an agent", Body: "Do the thing.", RepoId: seededRepoID(t),
	})
	require.NoError(t, err)

	browser := browserJobEnv(t)
	for range 3 {
		_, err = browser.tasks.RunTask(context.Background(),
			&factoryv1.RunTaskRequest{TaskId: created.Task.Id})
		assert.Equal(t, connect.CodeInvalidArgument, code(err))
	}
	assert.Len(t, browser.task(t, created.Task.Id).Comments, 1)
}

func TestClaimingMarksTheRunnerSeen(t *testing.T) {
	env := browserJobEnv(t)
	_, err := testDB.Exec(context.Background(),
		`UPDATE factory.runners SET last_seen = now() - interval '10 minutes'`)
	require.NoError(t, err)

	env.claim(t)

	var fresh bool
	require.NoError(t, testDB.QueryRow(context.Background(),
		`SELECT last_seen > now() - interval '1 minute' FROM factory.runners`).Scan(&fresh))
	assert.True(t, fresh)
}
