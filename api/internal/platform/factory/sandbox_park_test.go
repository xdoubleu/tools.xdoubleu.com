package factory_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
	"tools.xdoubleu.com/internal/platform/factory"
)

type digestFeed struct {
	value atomic.Value
	fail  atomic.Bool
}

func (d *digestFeed) set(v string) { d.value.Store(v) }

func (d *digestFeed) read(context.Context) (string, error) {
	if d.fail.Load() {
		return "", errors.New("manifest unreachable")
	}
	v, _ := d.value.Load().(string)
	return v, nil
}

func parkEnv(t *testing.T) (jobEnv, *digestFeed) {
	t.Helper()
	feed := &digestFeed{value: atomic.Value{}, fail: atomic.Bool{}}
	feed.set(uuid.NewString())
	env := newJobEnvWith(t, true, "browser", factory.WithImageDigestSource(feed.read))
	// Record the current digest (and settle parked tasks of other tests).
	require.NoError(t, env.app.ResumeParkedNow(context.Background()))
	return env, feed
}

// blockedJob runs a task to a sandbox-blocked completion with a session.
func (e jobEnv) blockedJob(t *testing.T, title string) string {
	t.Helper()
	task := e.newTask(t, title)
	e.run(t, task.Id, "")
	claimed := e.claim(t)
	_, err := e.runner.CompleteJob(bearer(e.token), &factoryv1.CompleteJobRequest{
		JobId: claimed.Id, SessionId: "sess-" + claimed.Id,
		Blocked: "no jq " + uuid.NewString(),
	})
	require.NoError(t, err)
	return task.Id
}

func TestASandboxBlockedTaskParksAndResumesOnANewImage(t *testing.T) {
	env, feed := parkEnv(t)
	id := env.blockedJob(t, "needs jq")

	got := env.task(t, id)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_BACKLOG, got.Task.Status)
	assert.Contains(t, got.Task.Labels, "waiting-on-sandbox")

	// The same digest resumes nothing.
	require.NoError(t, env.app.ResumeParkedNow(context.Background()))
	assert.Nil(t, env.claim(t))

	feed.set(uuid.NewString())
	require.NoError(t, env.app.ResumeParkedNow(context.Background()))
	got = env.task(t, id)
	assert.NotContains(t, got.Task.Labels, "waiting-on-sandbox")
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_IN_PROGRESS, got.Task.Status)
	resumed := env.claim(t)
	require.NotNil(t, resumed)
	assert.Equal(t, id, resumed.TaskId)
	assert.Contains(t, resumed.Prompt, "job image has been updated")
	assert.NotEmpty(t, resumed.ResumeSessionId)
}

func TestParkingIsCappedAtTwoImageResumes(t *testing.T) {
	env, feed := parkEnv(t)
	id := env.blockedJob(t, "never fixed")
	for range 2 {
		feed.set(uuid.NewString())
		require.NoError(t, env.app.ResumeParkedNow(context.Background()))
		claimed := env.claim(t)
		require.NotNil(t, claimed)
		_, err := env.runner.CompleteJob(bearer(env.token), &factoryv1.CompleteJobRequest{
			JobId: claimed.Id, SessionId: "again", Blocked: "still no jq",
		})
		require.NoError(t, err)
	}
	got := env.task(t, id)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_NEEDS_YOU, got.Task.Status)
	assert.NotContains(t, got.Task.Labels, "waiting-on-sandbox")
}

func TestABlockedJobWithoutASessionIsNotParked(t *testing.T) {
	env, _ := parkEnv(t)
	noSession := env.newTask(t, "no session")
	env.run(t, noSession.Id, "")
	claimed := env.claim(t)
	_, err := env.runner.CompleteJob(bearer(env.token), &factoryv1.CompleteJobRequest{
		JobId: claimed.Id, Blocked: "no jq " + uuid.NewString(),
	})
	require.NoError(t, err)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_NEEDS_YOU,
		env.task(t, noSession.Id).Task.Status)
}

func TestAnUnreachableManifestKeepsParkedTasksParked(t *testing.T) {
	env, feed := parkEnv(t)
	id := env.blockedJob(t, "waits")
	feed.fail.Store(true)
	require.Error(t, env.app.ResumeParkedNow(context.Background()))
	assert.Contains(t, env.task(t, id).Task.Labels, "waiting-on-sandbox")
	assert.Nil(t, env.claim(t))
}

func TestTheFirstParkRecordsTheDigestSoTheNextImageResumesIt(t *testing.T) {
	env, feed := parkEnv(t)
	_, err := testDB.Exec(context.Background(),
		"UPDATE factory.board_settings SET image_digest = NULL")
	require.NoError(t, err)
	id := env.blockedJob(t, "parked before any digest")

	feed.set(uuid.NewString())
	require.NoError(t, env.app.ResumeParkedNow(context.Background()))
	assert.NotContains(t, env.task(t, id).Task.Labels, "waiting-on-sandbox")
	require.NotNil(t, env.claim(t))
}

func TestAParkedTaskWithAJobAlreadyQueuedIsOnlyUnparked(t *testing.T) {
	env, feed := parkEnv(t)
	id := env.blockedJob(t, "already requeued")
	_, err := env.reply(t, id, "carry on")
	require.NoError(t, err)

	feed.set(uuid.NewString())
	require.NoError(t, env.app.ResumeParkedNow(context.Background()))
	assert.NotContains(t, env.task(t, id).Task.Labels, "waiting-on-sandbox")
	claimed := env.claim(t)
	require.NotNil(t, claimed)
	assert.Equal(t, "carry on", claimed.Prompt)
	assert.Nil(t, env.claim(t))
}
