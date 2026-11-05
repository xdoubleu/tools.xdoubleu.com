package factory_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
	"tools.xdoubleu.com/internal/githubapp"
)

func ended(state string) githubapp.PullRequestStatus {
	return githubapp.PullRequestStatus{State: state, CI: "", Review: ""}
}

func pullURL(number int) string {
	return fmt.Sprintf("https://github.com/xdoubleu/tools.xdoubleu.com/pull/%d", number)
}

// inReview runs a task to the end with a pull request, leaving it In review.
func inReview(t *testing.T, env jobEnv, title string, number int) *factoryv1.Task {
	t.Helper()
	task := env.newTask(t, title)
	env.run(t, task.Id, "")
	job := env.claim(t)
	complete(t, env, job.Id, true, pullURL(number))
	require.Equal(t, factoryv1.TaskStatus_TASK_STATUS_IN_REVIEW,
		env.task(t, task.Id).Task.Status)
	return task
}

func (e jobEnv) poll(t *testing.T) error {
	t.Helper()
	return e.app.PollPullRequestsNow(context.Background())
}

func (gh *fakeGitHub) answer(
	number int,
	status githubapp.PullRequestStatus,
	err error,
) {
	gh.mu.Lock()
	defer gh.mu.Unlock()
	gh.pulls[number] = pullAnswer{status: status, err: err}
}

func TestAnOpenPullRequestsChecksAndReviewAreKeptOnItsJob(t *testing.T) {
	env, gh := githubEnv(t)
	task := inReview(t, env, "open", 42)
	gh.answer(42, githubapp.PullRequestStatus{
		State: githubapp.StateOpen, CI: githubapp.CIFailing,
		Review: githubapp.ReviewChangesRequested,
	}, nil)

	require.NoError(t, env.poll(t))

	got := env.task(t, task.Id)
	job := got.Jobs[0]
	assert.Equal(t, "open", job.PrState)
	assert.Equal(t, "failing", job.PrCi)
	assert.Equal(t, "changes_requested", job.PrReview)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_IN_REVIEW, got.Task.Status)
	assert.Len(t, got.Comments, 1, "only the job's own comment")

	// It stays on the worklist, and the next read replaces the last.
	gh.answer(42, githubapp.PullRequestStatus{
		State: githubapp.StateOpen, CI: githubapp.CIPassing, Review: githubapp.ReviewApproved,
	}, nil)
	require.NoError(t, env.poll(t))
	job = env.task(t, task.Id).Jobs[0]
	assert.Equal(t, "passing", job.PrCi)
	assert.Equal(t, "approved", job.PrReview)
	assert.Equal(t, []int{42, 42}, gh.polled)
}

func TestAMergedPullRequestMovesItsTaskToTestingOnce(t *testing.T) {
	env, gh := githubEnv(t)
	task := inReview(t, env, "merged", 42)
	gh.answer(42, ended(githubapp.StateMerged), nil)

	require.NoError(t, env.poll(t))

	got := env.task(t, task.Id)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_TESTING, got.Task.Status)
	assert.Equal(t, "merged", got.Jobs[0].PrState)
	require.Len(t, got.Comments, 2)
	assert.Equal(t, "system:factory", got.Comments[1].Author)
	assert.Contains(t, got.Comments[1].Body, "was merged: "+pullURL(42))

	// A finished pull request is not read again.
	require.NoError(t, env.poll(t))
	assert.Equal(t, []int{42}, gh.polled)
	assert.Len(t, env.task(t, task.Id).Comments, 2)
}

func TestAPullRequestClosedWithoutMergingSendsItsTaskBackToNeedsYou(t *testing.T) {
	env, gh := githubEnv(t)
	task := inReview(t, env, "closed", 42)
	gh.answer(42, ended(githubapp.StateClosed), nil)

	require.NoError(t, env.poll(t))

	got := env.task(t, task.Id)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_NEEDS_YOU, got.Task.Status)
	assert.Equal(t, "closed", got.Jobs[0].PrState)
	assert.Contains(t, got.Comments[1].Body, "closed without merging")
}

func TestAPullRequestGitHubCannotFindIsNotCalledClosed(t *testing.T) {
	env, gh := githubEnv(t)
	task := inReview(t, env, "unseen", 42)
	gh.answer(42, ended(""), githubapp.ErrPullRequestGone)

	err := env.poll(t)
	require.ErrorIs(t, err, githubapp.ErrPullRequestGone)

	// The App may have lost sight of it (a rename, changed access): the task stays
	// where it is, and the pull request is tracked on.
	got := env.task(t, task.Id)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_IN_REVIEW, got.Task.Status)
	assert.Empty(t, got.Jobs[0].PrState)
	assert.Len(t, got.Comments, 1)
}

func TestAFailingPullRequestTakesItsTurnAtTheBackOfTheBatch(t *testing.T) {
	env, gh := githubEnv(t)
	task := inReview(t, env, "stuck", 42)
	gh.answer(42, ended(""), errors.New("status 502"))

	var checked bool
	require.NoError(t, testDB.QueryRow(context.Background(),
		`SELECT pr_checked_at IS NOT NULL FROM factory.jobs WHERE task_id = $1`,
		task.Id).Scan(&checked))
	require.False(t, checked, "never read")

	require.Error(t, env.poll(t))

	require.NoError(t, testDB.QueryRow(context.Background(),
		`SELECT pr_checked_at IS NOT NULL FROM factory.jobs WHERE task_id = $1`,
		task.Id).Scan(&checked))
	assert.True(t, checked, "tried, so the ones not yet tried go first next time")
}

func TestATaskAPersonMovedOnIsLeftWhereTheyPutIt(t *testing.T) {
	env, gh := githubEnv(t)
	task := inReview(t, env, "moved", 42)
	_, err := env.tasks.MoveTask(context.Background(), &factoryv1.MoveTaskRequest{
		Id: task.Id, Status: factoryv1.TaskStatus_TASK_STATUS_BACKLOG,
	})
	require.NoError(t, err)
	gh.answer(42, ended(githubapp.StateMerged), nil)

	require.NoError(t, env.poll(t))

	got := env.task(t, task.Id)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_BACKLOG, got.Task.Status)
	assert.Len(t, got.Comments, 1, "no comment either")
	assert.Equal(t, "merged", got.Jobs[0].PrState, "but it is recorded")
}

func TestAnOlderJobsPullRequestDoesNotMoveATaskALaterJobIsOn(t *testing.T) {
	env, gh := githubEnv(t)
	task := inReview(t, env, "again", 42)
	// A second run on the same task, then put back in review by hand.
	env.run(t, task.Id, "once more")
	_, err := env.tasks.MoveTask(context.Background(), &factoryv1.MoveTaskRequest{
		Id: task.Id, Status: factoryv1.TaskStatus_TASK_STATUS_IN_REVIEW,
	})
	require.NoError(t, err)
	gh.answer(42, ended(githubapp.StateMerged), nil)

	require.NoError(t, env.poll(t))

	got := env.task(t, task.Id)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_IN_REVIEW, got.Task.Status)
	assert.Equal(t, "merged", got.Jobs[1].PrState)
}

func TestOnePullRequestFailingDoesNotStopTheOthers(t *testing.T) {
	env, gh := githubEnv(t)
	broken := inReview(t, env, "broken", 41)
	fine := inReview(t, env, "fine", 42)
	gh.answer(41, ended(""), errors.New("status 502"))
	gh.answer(42, ended(githubapp.StateMerged), nil)

	err := env.poll(t)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 502")

	assert.Equal(
		t,
		factoryv1.TaskStatus_TASK_STATUS_TESTING,
		env.task(t, fine.Id).Task.Status,
	)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_IN_REVIEW,
		env.task(t, broken.Id).Task.Status)
	assert.Empty(t, env.task(t, broken.Id).Jobs[0].PrState, "unread, so tried again")

	// And the failing one is tried again.
	gh.answer(41, ended(githubapp.StateClosed), nil)
	require.NoError(t, env.poll(t))
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_NEEDS_YOU,
		env.task(t, broken.Id).Task.Status)
}

func TestWithoutAGitHubAppNothingIsPolled(t *testing.T) {
	env := browserJobEnv(t)
	task := env.newTask(t, "no app")
	env.run(t, task.Id, "")
	complete(t, env, env.claim(t).Id, true, pullURL(42))

	require.NoError(t, env.poll(t))
	got := env.task(t, task.Id)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_IN_REVIEW, got.Task.Status)
	assert.Empty(t, got.Jobs[0].PrState)
}

func TestAStoredPullRequestAddressOfAnotherRepoIsNeverAsked(t *testing.T) {
	env, gh := githubEnv(t)
	task := inReview(t, env, "tampered", 42)
	_, err := testDB.Exec(context.Background(), `
		UPDATE factory.jobs SET pr_url = 'https://github.com/someone/else/pull/9'
		WHERE task_id = $1`, task.Id)
	require.NoError(t, err)

	err = env.poll(t)
	require.ErrorContains(t, err, "not this repo's")
	assert.Empty(t, gh.polled)
}

func TestATasksPullRequestStatusIsOnTheBoardAndTheTask(t *testing.T) {
	env, gh := githubEnv(t)
	plain := env.newTask(t, "no pull request")
	task := inReview(t, env, "with one", 7)
	gh.answer(7, githubapp.PullRequestStatus{
		State: githubapp.StateOpen, CI: githubapp.CIFailing,
		Review: githubapp.ReviewApproved,
	}, nil)
	require.NoError(t, env.poll(t))

	list, err := env.tasks.ListTasks(context.Background(), &factoryv1.ListTasksRequest{})
	require.NoError(t, err)
	byID := map[string]*factoryv1.Task{}
	for _, listed := range list.Tasks {
		byID[listed.Id] = listed
	}
	assert.Empty(t, byID[plain.Id].PrUrl, "a task without a pull request")
	assert.Empty(t, byID[plain.Id].PrState)
	with := byID[task.Id]
	assert.Equal(t, pullURL(7), with.PrUrl)
	assert.Equal(t, "open", with.PrState)
	assert.Equal(t, "failing", with.PrCi)
	assert.Equal(t, "approved", with.PrReview)
	assert.Equal(t, with.PrCi, env.task(t, task.Id).Task.PrCi, "GetTask says the same")

	// A newer job that opened no pull request doesn't hide the older one's.
	env.run(t, task.Id, "")
	assert.Equal(t, pullURL(7), env.task(t, task.Id).Task.PrUrl)
}
