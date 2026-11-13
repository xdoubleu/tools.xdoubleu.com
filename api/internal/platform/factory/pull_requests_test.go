package factory_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
	"tools.xdoubleu.com/internal/platform/factory/internal/services"
	"tools.xdoubleu.com/internal/platform/githubapp"
)

func ended(state string) githubapp.PullRequestStatus {
	return githubapp.PullRequestStatus{State: state, CI: "", Review: "", Mergeable: "", MergeSHA: ""}
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
		Review: githubapp.ReviewChangesRequested, Mergeable: "", MergeSHA: "",
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
		Mergeable: "", MergeSHA: "",
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
		taskKey(t, task.Id)).Scan(&checked))
	require.False(t, checked, "never read")

	require.Error(t, env.poll(t))

	require.NoError(t, testDB.QueryRow(context.Background(),
		`SELECT pr_checked_at IS NOT NULL FROM factory.jobs WHERE task_id = $1`,
		taskKey(t, task.Id)).Scan(&checked))
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
		WHERE task_id = $1`, taskKey(t, task.Id))
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
		Review: githubapp.ReviewApproved, Mergeable: "", MergeSHA: "",
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

func (e jobEnv) putInReview(t *testing.T, id string) {
	t.Helper()
	_, err := e.tasks.MoveTask(context.Background(), &factoryv1.MoveTaskRequest{
		Id: id, Status: factoryv1.TaskStatus_TASK_STATUS_IN_REVIEW,
	})
	require.NoError(t, err)
}

func (gh *fakeGitHub) merge(url, body string) {
	gh.mergeAs(url, body, "abc123")
}

func (gh *fakeGitHub) mergeAs(url, body, sha string) {
	gh.mu.Lock()
	defer gh.mu.Unlock()
	gh.merged = append(gh.merged, githubapp.MergedPullRequest{
		URL: url, Body: body, MergeSHA: sha,
	})
}

func TestAMergedPullRequestLinkingAJoblessTaskMovesItToTestingOnce(t *testing.T) {
	env, gh := githubEnv(t)
	task := env.newTask(t, "local")
	env.putInReview(t, task.Id)
	gh.merge(pullURL(7), "Task: local ("+task.Id+")\n\nbody")

	require.NoError(t, env.poll(t))

	got := env.task(t, task.Id)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_TESTING, got.Task.Status)
	require.Len(t, got.Comments, 1)
	assert.Contains(t, got.Comments[0].Body, "was merged: "+pullURL(7))

	require.NoError(t, env.poll(t))
	assert.Len(t, env.task(t, task.Id).Comments, 1)
}

func TestAMergedLinkedPullRequestLeavesATaskNotInReview(t *testing.T) {
	env, gh := githubEnv(t)
	task := env.newTask(t, "moved on")
	gh.merge(pullURL(8), "Task: moved on ("+task.Id+")")
	gh.merge(pullURL(9), "no link here")
	gh.merge(pullURL(10), "Task: gone (00000000-0000-0000-0000-000000000000)")

	require.NoError(t, env.poll(t))

	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_BACKLOG,
		env.task(t, task.Id).Task.Status)
}

func (gh *fakeGitHub) openLinked(url string, number int, body string) {
	gh.mu.Lock()
	defer gh.mu.Unlock()
	gh.open = append(gh.open,
		githubapp.OpenPullRequest{URL: url, Number: number, Body: body})
}

func TestAnOpenPullRequestLinkingATaskIsListedWithItsChecks(t *testing.T) {
	env, gh := githubEnv(t)
	task := env.newTask(t, "by hand")
	gh.openLinked(pullURL(11), 11, "Task: by hand ("+task.Id+")")
	gh.openLinked(pullURL(12), 12, "no link here")
	gh.answer(11, githubapp.PullRequestStatus{
		State: githubapp.StateOpen, CI: githubapp.CIPending, Review: "", Mergeable: "", MergeSHA: "",
	}, nil)

	require.NoError(t, env.poll(t))

	got := env.task(t, task.Id)
	require.Len(t, got.PullRequests, 1)
	assert.Equal(t, pullURL(11), got.PullRequests[0].Url)
	assert.Equal(t, "pending", got.PullRequests[0].Ci)
	assert.Equal(t, pullURL(11), got.Task.PrUrl, "the card links it too")

	// It leaves the open list once merged: the next poll reads where it ended.
	gh.mu.Lock()
	gh.open = nil
	gh.mu.Unlock()
	gh.answer(11, ended(githubapp.StateMerged), nil)
	require.NoError(t, env.poll(t))
	got = env.task(t, task.Id)
	require.Len(t, got.PullRequests, 1)
	assert.Equal(t, "merged", got.PullRequests[0].State)
}

func TestATasksJobAndLinkedPullRequestsAreAllListed(t *testing.T) {
	env, gh := githubEnv(t)
	task := inReview(t, env, "both", 42)
	gh.openLinked(pullURL(43), 43, "Task: both ("+task.Id+")")
	gh.openLinked(pullURL(42), 42, "Task: both ("+task.Id+")")

	require.NoError(t, env.poll(t))

	got := env.task(t, task.Id)
	urls := []string{}
	for _, p := range got.PullRequests {
		urls = append(urls, p.Url)
	}
	assert.ElementsMatch(t, []string{pullURL(42), pullURL(43)}, urls)
}

func TestAMergedTaskShowsWhetherItsMergeCommitIsDeployed(t *testing.T) {
	env, gh := githubEnv(t)
	live := env.newTask(t, "shipped")
	waiting := env.newTask(t, "waiting")
	env.putInReview(t, live.Id)
	env.putInReview(t, waiting.Id)
	gh.mergeAs(pullURL(7), "Task: shipped ("+live.Id+")\n", "live-sha")
	gh.mergeAs(pullURL(8), "Task: waiting ("+waiting.Id+")\n", "new-sha")
	gh.mu.Lock()
	gh.live["live-sha"] = true
	gh.mu.Unlock()
	require.NoError(t, env.poll(t))

	assert.Equal(t, "deployed", env.task(t, live.Id).Task.DeployState)
	assert.Equal(t, "pending", env.task(t, waiting.Id).Task.DeployState)
}

func TestADeployCheckThatFailsLeavesTheStateUnknown(t *testing.T) {
	env, gh := githubEnv(t)
	task := env.newTask(t, "unsure")
	env.putInReview(t, task.Id)
	gh.merge(pullURL(7), "Task: unsure ("+task.Id+")\n\nbody")
	gh.mu.Lock()
	gh.deployErr = errors.New("github is down")
	gh.mu.Unlock()
	require.NoError(t, env.poll(t))

	got := env.task(t, task.Id).Task
	assert.Equal(t, "unknown", got.DeployState)
	assert.Equal(t, services.ReasonCompareFailed, got.DeployReason)
}

func TestAJobsMergedPullRequestRecordsItsMergeCommit(t *testing.T) {
	env, gh := githubEnv(t)
	task := inReview(t, env, "runner", 42)
	status := ended(githubapp.StateMerged)
	status.MergeSHA = "job-sha"
	gh.answer(42, status, nil)
	gh.mu.Lock()
	gh.live["job-sha"] = true
	gh.mu.Unlock()

	require.NoError(t, env.poll(t))

	got := env.task(t, task.Id).Task
	assert.Equal(t, "deployed", got.DeployState)
	assert.Empty(t, got.DeployReason)
}

func TestAMergedTaskWithoutACommitGetsItBackfilled(t *testing.T) {
	env, gh := githubEnv(t)
	task := env.newTask(t, "old")
	env.putInReview(t, task.Id)
	gh.mergeAs(pullURL(9), "Task: old ("+task.Id+")", "")
	require.NoError(t, env.poll(t))
	assert.Equal(t, services.ReasonNoMergeCommit, env.task(t, task.Id).Task.DeployReason)

	gh.mu.Lock()
	gh.merged = nil
	gh.mu.Unlock()
	gh.mergeAs(pullURL(9), "Task: old ("+task.Id+")", "late-sha")
	gh.mu.Lock()
	gh.live["late-sha"] = true
	gh.mu.Unlock()
	require.NoError(t, env.poll(t))

	assert.Equal(t, "deployed", env.task(t, task.Id).Task.DeployState)
}

func TestASlowDeployCheckDoesNotHoldUpTheBoard(t *testing.T) {
	env, gh := githubEnv(t)
	task := env.newTask(t, "slow")
	env.putInReview(t, task.Id)
	gh.merge(pullURL(7), "Task: slow ("+task.Id+")\n\nbody")
	require.NoError(t, env.poll(t))
	gh.mu.Lock()
	gh.deployHangs = true
	gh.mu.Unlock()

	start := time.Now()
	got := env.task(t, task.Id)

	assert.Less(t, time.Since(start), 5*time.Second)
	assert.Empty(t, got.Task.DeployState)
}

func TestATaskWithoutAMergeCommitHasNoDeployState(t *testing.T) {
	env, _ := githubEnv(t)
	task := env.newTask(t, "unmerged")

	assert.Empty(t, env.task(t, task.Id).Task.DeployState)
}

func TestAnArmedPullRequestThatCannotMergeIsFlaggedOnItsTaskAndCounted(t *testing.T) {
	env, gh := githubEnv(t)
	ctx := context.Background()
	task := env.newTask(t, "conflicting")
	gh.openLinked(pullURL(61), 61, "Task: conflicting ("+task.Id+")")
	gh.answer(61, githubapp.PullRequestStatus{
		State: githubapp.StateOpen, CI: githubapp.CIPassing, Review: "",
		Mergeable: githubapp.MergeConflicting,
	}, nil)
	before := gaugeValue(t, "factory_pull_requests_unmergeable")

	require.NoError(t, env.poll(t))

	got := env.task(t, task.Id)
	assert.Equal(t, "conflicting", got.Task.PrMergeable)
	require.Len(t, got.PullRequests, 1)
	assert.Equal(t, "conflicting", got.PullRequests[0].Mergeable)

	// Fresh, it is not yet counted; after an hour it is, and it stays counted.
	require.NoError(t, env.app.CollectSignalsNow(ctx))
	assert.InDelta(t, before, gaugeValue(t, "factory_pull_requests_unmergeable"), 0)
	_, err := testDB.Exec(ctx, `UPDATE factory.linked_pull_requests
		SET unmergeable_since = now() - interval '2 hours' WHERE url = $1`, pullURL(61))
	require.NoError(t, err)
	require.NoError(t, env.poll(t))
	require.NoError(t, env.app.CollectSignalsNow(ctx))
	assert.Greater(t, gaugeValue(t, "factory_pull_requests_unmergeable"), before)

	// Once it can merge, the flag clears.
	gh.answer(61, githubapp.PullRequestStatus{
		State: githubapp.StateOpen, CI: githubapp.CIPassing, Review: "", Mergeable: "",
	}, nil)
	require.NoError(t, env.poll(t))
	assert.Empty(t, env.task(t, task.Id).Task.PrMergeable)
}
