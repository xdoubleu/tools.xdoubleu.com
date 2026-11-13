package factory_test

import (
	"context"
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
)

// repoDefaults sets the seeded repo's default harness and model for one test, and
// puts Claude Code back afterwards.
func repoDefaults(t *testing.T, harness, model string) {
	t.Helper()
	set := func(h, m string) {
		_, err := testDB.Exec(context.Background(),
			`UPDATE factory.repos SET default_harness = $1, default_model = $2
			 WHERE id = $3`, h, m, seededRepoID(t))
		require.NoError(t, err)
	}
	set(harness, model)
	t.Cleanup(func() { set("claude_code", "") })
}

func (e jobEnv) runWith(
	t *testing.T,
	taskID string,
	harness factoryv1.Harness,
	model string,
) (*factoryv1.Job, error) {
	t.Helper()
	resp, err := e.tasks.RunTask(context.Background(), &factoryv1.RunTaskRequest{
		TaskId: taskID, Harness: harness, Model: model,
	})
	if err != nil {
		return nil, err
	}
	return resp.Job, nil
}

func TestAJobRunsOnTheReposDefaultHarnessUnlessPicked(t *testing.T) {
	env := browserJobEnv(t)
	repoDefaults(t, "opencode", "vendor/default")

	job := env.run(t, env.newTask(t, "default").Id, "")
	assert.Equal(t, factoryv1.Harness_HARNESS_OPENCODE, job.Harness)
	assert.Equal(t, "vendor/default", job.Model)

	claimed := env.claim(t)
	assert.Equal(t, factoryv1.Harness_HARNESS_OPENCODE, claimed.Harness)
	assert.Equal(t, "vendor/default", claimed.Model)
}

func TestAPickedHarnessOverridesTheDefault(t *testing.T) {
	env := browserJobEnv(t)
	repoDefaults(t, "opencode", "vendor/default")

	claude, err := env.runWith(t, env.newTask(t, "claude").Id,
		factoryv1.Harness_HARNESS_CLAUDE_CODE, "")
	require.NoError(t, err)
	assert.Equal(t, factoryv1.Harness_HARNESS_CLAUDE_CODE, claude.Harness)
	assert.Empty(t, claude.Model, "Claude Code picks its own model")

	other, err := env.runWith(t, env.newTask(t, "model").Id,
		factoryv1.Harness_HARNESS_OPENCODE, " vendor/other:free ")
	require.NoError(t, err)
	assert.Equal(t, "vendor/other:free", other.Model)

	same, err := env.runWith(t, env.newTask(t, "just model").Id,
		factoryv1.Harness_HARNESS_UNSPECIFIED, "vendor/third")
	require.NoError(t, err)
	assert.Equal(t, factoryv1.Harness_HARNESS_OPENCODE, same.Harness)
	assert.Equal(t, "vendor/third", same.Model, "a model alone changes only the model")
}

func TestPickingOpenCodeNeedsAModelFromSomewhere(t *testing.T) {
	env := browserJobEnv(t)

	// The repo runs Claude Code, so there is no default model to fall back on.
	_, err := env.runWith(t, env.newTask(t, "no model").Id,
		factoryv1.Harness_HARNESS_OPENCODE, "")
	assert.Equal(t, connect.CodeInvalidArgument, code(err))

	// With an OpenCode repo, the default model fills in.
	repoDefaults(t, "opencode", "vendor/default")
	job, err := env.runWith(t, env.newTask(t, "falls back").Id,
		factoryv1.Harness_HARNESS_OPENCODE, "")
	require.NoError(t, err)
	assert.Equal(t, "vendor/default", job.Model)
}

func TestARunWithABadPickIsRefused(t *testing.T) {
	env := browserJobEnv(t)
	task := env.newTask(t, "bad")

	for name, tc := range map[string]struct {
		harness factoryv1.Harness
		model   string
	}{
		"claude code with a model": {factoryv1.Harness_HARNESS_CLAUDE_CODE, "vendor/model"},
		"model as an option":       {factoryv1.Harness_HARNESS_OPENCODE, "-x/model"},
		"model without a vendor":   {factoryv1.Harness_HARNESS_OPENCODE, "model"},
		"unknown harness":          {factoryv1.Harness(99), ""},
	} {
		_, err := env.runWith(t, task.Id, tc.harness, tc.model)
		assert.Equal(t, connect.CodeInvalidArgument, code(err), name)
	}
	assert.Empty(t, env.task(t, task.Id).Jobs, "nothing was queued")
}

func TestAReplyKeepsTheHarnessAndModelOfTheJobItContinues(t *testing.T) {
	env := browserJobEnv(t)
	repoDefaults(t, "opencode", "vendor/default")
	task := env.newTask(t, "continues")
	first := env.finishedJob(t, task.Id)
	assert.Equal(t, "vendor/default", first.Model)

	// The repo's default moves on; the session belongs to the old harness.
	repoDefaults(t, "claude_code", "")
	_, err := env.reply(t, task.Id, "more")
	require.NoError(t, err)
	second := env.claim(t)
	assert.Equal(t, factoryv1.Harness_HARNESS_OPENCODE, second.Harness)
	assert.Equal(t, "vendor/default", second.Model)
}
