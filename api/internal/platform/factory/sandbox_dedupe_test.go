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
	env.run(t, task.Id, "")
	claimed := env.claim(t)
	_, err := env.runner.CompleteJob(bearer(env.token), &factoryv1.CompleteJobRequest{
		JobId: claimed.Id, Blocked: blocked,
	})
	require.NoError(t, err)
}

func TestABlockedLineNamingAnOpenTaskCommentsOnThatTask(t *testing.T) {
	env, _ := accountsEnv(t)
	known := env.newTask(t, "host gate has no go")
	blocked := "host gate has no go (" + known.Id + ")"
	blockWith(t, env, blocked)
	blockWith(t, env, blocked)

	assert.Empty(t, sandboxTasks(t, env, "Sandbox: "+blocked))
	got := env.task(t, known.Id)
	require.Len(t, got.Comments, 2)
	assert.Contains(t, got.Comments[0].Body, "host gate has no go")
}

func TestABlockedLineWithTheSameCauseReusesTheOpenSandboxTask(t *testing.T) {
	env, _ := accountsEnv(t)
	tag := uuid.NewString()[:8]
	first := "Host gate has no go toolchain " + tag
	blockWith(t, env, first)
	blockWith(t, env, "the host gate has no go toolchain for "+tag)

	found := sandboxTasks(t, env, "Sandbox: "+first)
	require.Len(t, found, 1)
	assert.Len(t, env.task(t, found[0].Id).Comments, 2)
}
