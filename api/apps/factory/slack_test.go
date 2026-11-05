package factory_test

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/factory"
	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
)

const boardURL = "https://tools.example.com"

type fakeSlack struct {
	mu    sync.Mutex
	texts []string
}

func (f *fakeSlack) Notify(_ context.Context, text string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.texts = append(f.texts, text)
}

func (f *fakeSlack) last() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.texts) == 0 {
		return ""
	}
	return f.texts[len(f.texts)-1]
}

func slackEnv(t *testing.T) (jobEnv, *fakeSlack) {
	t.Helper()
	slack := &fakeSlack{mu: sync.Mutex{}, texts: nil}
	return newJobEnvWith(t, true, "browser", factory.WithNotifier(slack, boardURL)), slack
}

func TestPairingARunnerIsPosted(t *testing.T) {
	_, slack := slackEnv(t)
	require.NotEmpty(t, slack.texts)
	assert.Equal(t, ":link: A runner paired: Home Mac", slack.texts[0])
}

func TestAJobsPullRequestIsPostedWithALinkToItsTask(t *testing.T) {
	env, slack := slackEnv(t)
	task := env.newTask(t, "Fix the login")
	env.run(t, task.Id, "")
	job := env.claim(t)

	complete(t, env, job.Id, true, pullURL(5))

	assert.Equal(t, ":white_check_mark: Pull request opened for <"+boardURL+
		"/factory?task="+task.Id+"|Fix the login>: <"+pullURL(5)+"|review it>", slack.last())
}

func TestAFailedJobIsPostedWithOnlyTheFirstLineOfItsOutcome(t *testing.T) {
	env, slack := slackEnv(t)
	task := env.newTask(t, "Breaks")
	env.run(t, task.Id, "")
	job := env.claim(t)

	_, err := env.runner.CompleteJob(bearer(env.token), &factoryv1.CompleteJobRequest{
		JobId: job.Id, Succeeded: false, Outcome: "Tests failed.\nsecret log line",
	})
	require.NoError(t, err)

	assert.Contains(
		t,
		slack.last(),
		":x: A job failed on <"+boardURL+"/factory?task="+task.Id,
	)
	assert.Contains(t, slack.last(), "Tests failed.")
	assert.NotContains(t, slack.last(), "secret log line")
}

func TestASucceededJobWithoutAPullRequestNeedsYou(t *testing.T) {
	env, slack := slackEnv(t)
	task := env.newTask(t, "No pr")
	env.run(t, task.Id, "")
	job := env.claim(t)

	_, err := env.runner.CompleteJob(bearer(env.token), &factoryv1.CompleteJobRequest{
		JobId: job.Id, Succeeded: true, Outcome: "Left it on the Mac.",
	})
	require.NoError(t, err)

	assert.Contains(t, slack.last(), ":warning: A job finished with no pull request")
	assert.Contains(t, slack.last(), "which needs you: Left it on the Mac.")
}

func TestALostJobIsPosted(t *testing.T) {
	env, slack := slackEnv(t)
	task := env.newTask(t, "vanishes")
	env.run(t, task.Id, "")
	claimed := env.claim(t)
	_, err := testDB.Exec(context.Background(),
		`UPDATE factory.jobs SET last_check_in = now() - interval '6 minutes'
		 WHERE id = $1`, claimed.Id)
	require.NoError(t, err)

	_, err = env.tasks.ListTasks(context.Background(), &factoryv1.ListTasksRequest{})
	require.NoError(t, err)

	assert.Contains(
		t,
		slack.last(),
		":x: A job was lost on <"+boardURL+"/factory?task="+task.Id,
	)
	assert.Contains(t, slack.last(), "the runner stopped checking in")
}

func TestATitleOrOutcomeCannotMentionTheChannel(t *testing.T) {
	env, slack := slackEnv(t)
	task := env.newTask(t, "<!channel> look")
	env.run(t, task.Id, "")
	job := env.claim(t)

	_, err := env.runner.CompleteJob(bearer(env.token), &factoryv1.CompleteJobRequest{
		JobId: job.Id, Succeeded: false, Outcome: "<!everyone> & <@U123>",
	})
	require.NoError(t, err)

	text := slack.last()
	assert.NotContains(t, text, "<!channel>")
	assert.NotContains(t, text, "<!everyone>")
	assert.NotContains(t, text, "<@U123>")
	assert.True(t, strings.Contains(text, "&lt;!channel&gt; look"))
	assert.Contains(t, text, "&lt;!everyone&gt; &amp; &lt;@U123&gt;")
}

func TestAWebhookAddressIsAcceptedOnlyOverHTTPS(t *testing.T) {
	for _, url := range []string{
		"", "http://hooks.slack.com/x", "https://hooks.slack.com/x",
	} {
		env := newJobEnvWith(t, true, "browser", factory.WithSlack(url, boardURL))
		task := env.newTask(t, "quiet")
		env.run(t, task.Id, "")
		job := env.claim(t)
		complete(t, env, job.Id, true, pullURL(1))
	}
}
