package factory_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
)

func seededRepoSlug(t *testing.T) string {
	t.Helper()
	var slug string
	require.NoError(t, testDB.QueryRow(context.Background(),
		`SELECT owner || '/' || name FROM factory.repos WHERE id = $1`,
		seededRepoID(t)).Scan(&slug))
	return slug
}

func runnerRepos(t *testing.T, repos []string) {
	t.Helper()
	_, err := testDB.Exec(context.Background(),
		`UPDATE factory.runners SET repos = $1`, repos)
	require.NoError(t, err)
}

func (e jobEnv) comments(t *testing.T, taskID string) []string {
	t.Helper()
	var out []string
	for _, c := range e.task(t, taskID).Comments {
		out = append(out, c.Body)
	}
	return out
}

func TestHeartbeatStoresTheRunnersAllowlist(t *testing.T) {
	env := browserJobEnv(t)
	_, err := env.runner.Heartbeat(bearer(env.token), &factoryv1.HeartbeatRequest{
		Repos: []string{"a/b"}, ReposReported: true,
	})
	require.NoError(t, err)
	var repos []string
	require.NoError(t, testDB.QueryRow(context.Background(),
		`SELECT repos FROM factory.runners WHERE revoked_at IS NULL`).Scan(&repos))
	assert.Equal(t, []string{"a/b"}, repos)

	// A heartbeat that reports nothing leaves the stored list alone.
	_, err = env.runner.Heartbeat(bearer(env.token), &factoryv1.HeartbeatRequest{})
	require.NoError(t, err)
	require.NoError(t, testDB.QueryRow(context.Background(),
		`SELECT repos FROM factory.runners WHERE revoked_at IS NULL`).Scan(&repos))
	assert.Equal(t, []string{"a/b"}, repos)
}

func TestAReadyTaskNoRunnerAllowsIsCommentedOnceAndNotPickedUp(t *testing.T) {
	env := browserJobEnv(t)
	autoPickup(t, true)
	task := env.newTask(t, "unallowed")
	env.ready(t, task.Id)
	runnerRepos(t, []string{"someone/else"})

	assert.Nil(t, env.claim(t))
	assert.Nil(t, env.claim(t))
	comments := env.comments(t, task.Id)
	require.Len(t, comments, 1)
	assert.Contains(t, comments[0], "No runner allows "+seededRepoSlug(t))
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_READY,
		env.task(t, task.Id).Task.Status)

	// Once the runner allows the repo, the task is picked up.
	runnerRepos(t, []string{seededRepoSlug(t)})
	claimed := env.claim(t)
	require.NotNil(t, claimed)
	assert.Equal(t, task.Id, claimed.TaskId)
}

func TestAClaimNeverHandsARunnerAJobForARepoItDoesNotAllow(t *testing.T) {
	env := browserJobEnv(t)
	task := env.newTask(t, "queued")
	env.run(t, task.Id)

	runnerRepos(t, []string{})
	assert.Nil(t, env.claim(t))
	runnerRepos(t, []string{"SOMEONE/else", seededRepoSlug(t)})
	require.NotNil(t, env.claim(t))
}

func TestAReadyTaskOfARepoWithAutoPickupOffGetsAComment(t *testing.T) {
	env := browserJobEnv(t)
	autoPickup(t, false)
	task := env.newTask(t, "off")
	env.ready(t, task.Id)

	assert.Nil(t, env.claim(t))
	assert.Nil(t, env.claim(t))
	comments := env.comments(t, task.Id)
	require.Len(t, comments, 1)
	assert.Contains(t, comments[0], "auto pickup is off for "+seededRepoSlug(t))

	_, err := testDB.Exec(context.Background(),
		`UPDATE factory.repos SET auto_pickup = true WHERE id = $1`, seededRepoID(t))
	require.NoError(t, err)
	require.Eventually(t, func() bool { return env.claim(t) != nil },
		5*time.Second, 50*time.Millisecond)
}

// withoutPickupNotes drops the comments auto pickup leaves on a Ready task it
// passes over, for tests that count the comments of another feature.
func withoutPickupNotes(all []*factoryv1.Comment) []*factoryv1.Comment {
	var out []*factoryv1.Comment
	for _, c := range all {
		if c.Author != "system:auto-pickup" {
			out = append(out, c)
		}
	}
	return out
}
