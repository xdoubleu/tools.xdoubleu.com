package factory_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
	"tools.xdoubleu.com/internal/models"
)

func sandboxTasks(t *testing.T, env jobEnv, title string) []*factoryv1.Task {
	t.Helper()
	resp, err := env.tasks.ListTasks(context.Background(), &factoryv1.ListTasksRequest{})
	require.NoError(t, err)
	var out []*factoryv1.Task
	for _, task := range resp.Tasks {
		for _, l := range task.Labels {
			if task.Title != title {
				continue
			}
			if l == "factory-sandbox" {
				out = append(out, task)
			}
		}
	}
	return out
}

func TestABlockedJobFilesASandboxTaskAndARepeatCommentsOnIt(t *testing.T) {
	env, _ := accountsEnv(t)
	problem := "no module cache " + uuid.NewString()
	title := "Sandbox: " + problem
	for range 2 {
		task := env.newTask(t, "some work")
		env.run(t, task.Id, "")
		claimed := env.claim(t)
		_, err := env.runner.CompleteJob(bearer(env.token), &factoryv1.CompleteJobRequest{
			JobId: claimed.Id, Blocked: problem + "\nsecond line",
		})
		require.NoError(t, err)
	}
	found := sandboxTasks(t, env, title)
	require.Len(t, found, 1)
	assert.Equal(t, title, found[0].Title)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_BACKLOG, found[0].Status)
	got := env.task(t, found[0].Id)
	require.Len(t, got.Comments, 1)
	assert.Contains(t, got.Comments[0].Body, problem)

	// A job that is not blocked files nothing.
	task := env.newTask(t, "more work")
	env.run(t, task.Id, "")
	claimed := env.claim(t)
	_, err := env.runner.CompleteJob(bearer(env.token),
		&factoryv1.CompleteJobRequest{JobId: claimed.Id})
	require.NoError(t, err)
	assert.Len(t, sandboxTasks(t, env, title), 1)
}

func TestAJobAccountReportsToTheSandboxBoardOnly(t *testing.T) {
	env, accounts := accountsEnv(t)
	title := "Sandbox: no network " + uuid.NewString()
	mine := env.newTask(t, "my task")
	other := env.newTask(t, "unrelated task")
	env.run(t, mine.Id, "")
	claimed := env.claim(t)
	acc := accounts.of(t, claimed.Id)
	session := mcpSessionOn(t, env.app, &models.User{
		ID: acc.userID, Email: "job@agent.invalid", Role: models.RoleAgent,
		AppAccess: []string{"factory"}, WriteAccess: []string{"factory"},
		HasMFA: false, DisplayName: "",
	})

	args := map[string]any{
		"repo_id": seededRepoID(t), "title": title, "body": "first",
		"labels": []string{"p0", "other"}, "priority": "p0",
	}
	made, err := callTool(t, session, "factory_create_task", args)
	require.NoError(t, err)
	assert.Equal(t, "TASK_PRIORITY_P2", str(t, made, "task", "priority"))
	args["body"] = "second"
	again, err := callTool(t, session, "factory_create_task", args)
	require.NoError(t, err)
	id := str(t, made, "task", "id")
	assert.Equal(t, id, str(t, again, "task", "id"))
	found := sandboxTasks(t, env, title)
	require.Len(t, found, 1)
	assert.Equal(t, []string{"factory-sandbox"}, found[0].Labels)
	assert.Len(t, env.task(t, id).Comments, 1)

	// It sees and comments on the sandbox task, not on other people's.
	list, err := callTool(t, session, "factory_list_tasks", map[string]any{})
	require.NoError(t, err)
	tasks, ok := list["tasks"].([]any)
	require.True(t, ok)
	ids := []string{}
	for _, task := range tasks {
		ids = append(ids, str(t, task, "id"))
	}
	assert.Contains(t, ids, id)
	assert.Contains(t, ids, mine.Id)
	assert.NotContains(t, ids, other.Id)
	_, err = callTool(t, session, "factory_comment_task",
		map[string]any{"task_id": id, "body": "More detail."})
	require.NoError(t, err)
	assert.Len(t, env.task(t, id).Comments, 2)
	_, err = callTool(t, session, "factory_comment_task",
		map[string]any{"task_id": other.Id, "body": "Nope."})
	require.Error(t, err)
}
