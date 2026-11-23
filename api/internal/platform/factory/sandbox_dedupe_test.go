package factory_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
)

func blockWith(t *testing.T, env jobEnv, blocked string) {
	t.Helper()
	task := env.newTask(t, "work "+uuid.NewString())
	env.run(t, task.Id)
	claimed := env.claim(t)
	_, err := env.runner.CompleteJob(bearer(env.token), &factoryv1.CompleteJobRequest{
		JobId: claimed.Id, Blocked: blocked,
	})
	require.NoError(t, err)
}

func TestABlockedLineNamingATaskBacklogsTheTaskAndFilesNothing(t *testing.T) {
	env, _ := accountsEnv(t)
	known := env.newTask(t, "host gate has no go")
	blocked := "needs the filing routines (" + known.Id + ") " + uuid.NewString()
	blockWith(t, env, blocked)

	assert.Empty(t, sandboxTasks(t, env, "Sandbox: "+blocked))
}

func TestAWaitingOnJobMovesItsTaskToBacklogNamingTheDependencies(t *testing.T) {
	env, _ := accountsEnv(t)
	task := env.newTask(t, "work "+uuid.NewString())
	env.run(t, task.Id)
	claimed := env.claim(t)
	_, err := env.runner.CompleteJob(bearer(env.token), &factoryv1.CompleteJobRequest{
		JobId: claimed.Id, WaitingOn: []string{"T-99996", "see T-99998 and T-99996"},
	})
	require.NoError(t, err)

	got := env.task(t, task.Id)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_BACKLOG, got.Task.Status)
	require.NotEmpty(t, got.Comments)
	assert.Contains(t, lastComment(got), "Waiting on T-99996, T-99998;")
}

func TestABlockedLineForWorkThatDoesNotExistYetFilesNothing(t *testing.T) {
	env, _ := accountsEnv(t)
	blocked := "the repair path doesn't exist yet " + uuid.NewString()
	blockWith(t, env, blocked)

	assert.Empty(t, sandboxTasks(t, env, "Sandbox: "+blocked))
}

func TestABlockedLineWithTheSameCauseReusesTheOpenSandboxTask(t *testing.T) {
	env, _ := accountsEnv(t)
	tag := uuid.NewString()[:8]
	first := "Host gate has no go toolchain " + tag
	blockWith(t, env, first)
	blockWith(t, env, "the host gate has no go toolchain for "+tag)

	found := sandboxTasks(t, env, "Sandbox: "+first)
	require.Len(t, found, 1)
	assert.Len(t, withoutPickupNotes(env.task(t, found[0].Id).Comments), 1,
		"the repeat")
}

func TestAWaitingOnJobNamingItsOwnTaskDoesNotParkIt(t *testing.T) {
	env, _ := accountsEnv(t)
	task := env.newTask(t, "work "+uuid.NewString())
	env.run(t, task.Id)
	claimed := env.claim(t)
	_, err := env.runner.CompleteJob(bearer(env.token), &factoryv1.CompleteJobRequest{
		JobId: claimed.Id, WaitingOn: []string{task.Id},
	})
	require.NoError(t, err)

	got := env.task(t, task.Id)
	assert.NotEqual(t, factoryv1.TaskStatus_TASK_STATUS_BACKLOG, got.Task.Status)
	for _, c := range got.Comments {
		assert.NotContains(t, c.Body, "moved to Backlog")
	}
}
