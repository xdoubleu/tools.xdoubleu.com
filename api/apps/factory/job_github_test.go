package factory_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/factory"
	"tools.xdoubleu.com/apps/factory/internal/services"
	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
	"tools.xdoubleu.com/internal/githubapp"
)

const ownPull = "https://github.com/xdoubleu/tools.xdoubleu.com/pull/42"

// fakeGitHub records who a token was asked for and answers as told.
type fakeGitHub struct {
	mu    sync.Mutex
	asked [][2]string
	err   error
}

func (f *fakeGitHub) InstallationToken(
	_ context.Context, owner, repo string,
) (githubapp.Token, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked = append(f.asked, [2]string{owner, repo})
	if f.err != nil {
		return githubapp.Token{}, f.err
	}
	return githubapp.Token{
		Value:     "ghs_for_" + repo,
		ExpiresAt: time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC),
	}, nil
}

var _ services.JobGitHub = (*fakeGitHub)(nil)

func githubEnv(t *testing.T) (jobEnv, *fakeGitHub) {
	t.Helper()
	gh := &fakeGitHub{mu: sync.Mutex{}, asked: nil, err: nil}
	return newJobEnvWith(t, true, "browser", factory.WithGitHub(gh)), gh
}

func (e jobEnv) pushToken(
	t *testing.T,
	jobID string,
) (*factoryv1.GetPushTokenResponse, error) {
	t.Helper()
	return e.runner.GetPushToken(
		bearer(e.token),
		&factoryv1.GetPushTokenRequest{JobId: jobID},
	)
}

func TestARunningJobGetsATokenForItsRepoAlone(t *testing.T) {
	env, gh := githubEnv(t)
	env.run(t, env.newTask(t, "pushes").Id, "")
	job := env.claim(t)

	got, err := env.pushToken(t, job.Id)
	require.NoError(t, err)
	assert.Equal(t, "ghs_for_tools.xdoubleu.com", got.Token)
	assert.Equal(t, "2026-11-04T10:00:00Z", got.ExpiresAt)
	assert.Equal(t, [][2]string{{"xdoubleu", "tools.xdoubleu.com"}}, gh.asked)
}

func TestAPushTokenIsRefusedForAJobThatIsNotRunning(t *testing.T) {
	env, gh := githubEnv(t)
	task := env.newTask(t, "ends")
	env.run(t, task.Id, "")
	job := env.claim(t)
	_, err := env.stop(t, task.Id)
	require.NoError(t, err)

	_, err = env.pushToken(t, job.Id)
	assert.Equal(t, connect.CodeAlreadyExists, code(err))
	assert.Empty(t, gh.asked, "GitHub is not asked")

	_, err = env.pushToken(t, "nope")
	assert.Equal(t, connect.CodeInvalidArgument, code(err))
	_, err = env.pushToken(t, "11111111-2222-3333-4444-555555555555")
	assert.Equal(t, connect.CodeNotFound, code(err))

	_, err = env.runner.GetPushToken(context.Background(),
		&factoryv1.GetPushTokenRequest{JobId: job.Id})
	assert.Equal(t, connect.CodeUnauthenticated, code(err), "a device token is needed")
}

func TestWithoutAGitHubAppNothingCanPush(t *testing.T) {
	env := browserJobEnv(t)
	env.run(t, env.newTask(t, "stays on the mac").Id, "")
	job := env.claim(t)
	_, err := env.pushToken(t, job.Id)
	assert.Equal(t, connect.CodeFailedPrecondition, code(err))
}

func TestGitHubTroubleIsToldApartFromEachOther(t *testing.T) {
	env, gh := githubEnv(t)
	env.run(t, env.newTask(t, "unlucky").Id, "")
	job := env.claim(t)

	gh.err = githubapp.ErrNotInstalled
	_, err := env.pushToken(t, job.Id)
	assert.Equal(t, connect.CodeFailedPrecondition, code(err), "the App isn't installed")

	gh.err = errors.New("github: POST /app/installations/1/access_tokens: status 502")
	_, err = env.pushToken(t, job.Id)
	assert.Equal(t, connect.CodeInternal, code(err), "anything else is a failure to retry")
}

func complete(t *testing.T, env jobEnv, jobID string, ok bool, prURL string) {
	t.Helper()
	_, err := env.runner.CompleteJob(bearer(env.token), &factoryv1.CompleteJobRequest{
		JobId: jobID, Succeeded: ok, Outcome: "Did it.", PrUrl: prURL,
	})
	require.NoError(t, err)
}

func TestAJobThatOpenedAPullRequestMovesItsTaskToInReview(t *testing.T) {
	env := browserJobEnv(t)
	task := env.newTask(t, "gets a pr")
	env.run(t, task.Id, "")
	job := env.claim(t)

	complete(t, env, job.Id, true, ownPull)

	got := env.task(t, task.Id)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_IN_REVIEW, got.Task.Status)
	require.Len(t, got.Jobs, 1)
	assert.Equal(t, ownPull, got.Jobs[0].PrUrl)
	require.Len(t, got.Comments, 1)
	assert.Contains(t, got.Comments[0].Body, "pull request: "+ownPull)
	assert.Contains(t, got.Comments[0].Body, "Did it.")
	assert.NotContains(t, got.Comments[0].Body, "left out")
}

func TestAPullRequestLinkThatIsNotThisReposIsLeftOut(t *testing.T) {
	for name, link := range map[string]string{
		"another repo":   "https://github.com/xdoubleu/other/pull/42",
		"another owner":  "https://github.com/someone/tools.xdoubleu.com/pull/42",
		"another host":   "https://evil.example/xdoubleu/tools.xdoubleu.com/pull/42",
		"plain http":     "http://github.com/xdoubleu/tools.xdoubleu.com/pull/42",
		"not a pull":     "https://github.com/xdoubleu/tools.xdoubleu.com/issues/42",
		"pull zero":      "https://github.com/xdoubleu/tools.xdoubleu.com/pull/0",
		"extra path":     ownPull + "/files",
		"with a query":   ownPull + "?x=1",
		"with a hash":    ownPull + "#top",
		"with user info": "https://user@github.com/xdoubleu/tools.xdoubleu.com/pull/42",
		"not a url":      "%zz",
	} {
		env := browserJobEnv(t)
		task := env.newTask(t, "link "+name)
		env.run(t, task.Id, "")
		job := env.claim(t)

		complete(t, env, job.Id, true, link)

		got := env.task(t, task.Id)
		assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_NEEDS_YOU, got.Task.Status, name)
		assert.Empty(t, got.Jobs[0].PrUrl, name)
		assert.Contains(t, got.Comments[0].Body, "left out", name)
		assert.NotContains(t, got.Comments[0].Body, link, name)
	}
}

func TestTheRepoNameInALinkIsMatchedIgnoringCase(t *testing.T) {
	env := browserJobEnv(t)
	task := env.newTask(t, "case")
	env.run(t, task.Id, "")
	job := env.claim(t)
	complete(t, env, job.Id, true, "https://github.com/XDoubleu/Tools.XDoubleu.com/pull/7")
	assert.Equal(
		t,
		factoryv1.TaskStatus_TASK_STATUS_IN_REVIEW,
		env.task(t, task.Id).Task.Status,
	)
}

func TestAFailedJobKeepsNoPullRequestAndGoesToNeedsYou(t *testing.T) {
	env := browserJobEnv(t)
	task := env.newTask(t, "failed")
	env.run(t, task.Id, "")
	job := env.claim(t)

	complete(t, env, job.Id, false, ownPull)

	got := env.task(t, task.Id)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_NEEDS_YOU, got.Task.Status)
	assert.Empty(t, got.Jobs[0].PrUrl)
	assert.NotContains(t, got.Comments[0].Body, "left out", "nothing was reported to drop")
}

func TestAJobWithoutAPullRequestStillGoesToNeedsYou(t *testing.T) {
	env := browserJobEnv(t)
	task := env.newTask(t, "no pr")
	env.run(t, task.Id, "")
	job := env.claim(t)
	complete(t, env, job.Id, true, "")
	assert.Equal(
		t,
		factoryv1.TaskStatus_TASK_STATUS_NEEDS_YOU,
		env.task(t, task.Id).Task.Status,
	)
}

func TestCompletingAJobThatIsGoneStaysAConflictWithALink(t *testing.T) {
	env := browserJobEnv(t)
	task := env.newTask(t, "gone")
	env.run(t, task.Id, "")
	job := env.claim(t)
	_, err := testDB.Exec(
		context.Background(),
		`DELETE FROM factory.jobs WHERE id = $1`,
		job.Id,
	)
	require.NoError(t, err)

	_, err = env.runner.CompleteJob(bearer(env.token), &factoryv1.CompleteJobRequest{
		JobId: job.Id, Succeeded: true, PrUrl: ownPull,
	})
	assert.Equal(t, connect.CodeAlreadyExists, code(err))
}
