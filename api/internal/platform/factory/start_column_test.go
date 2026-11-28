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

func TestAgentTasksAreCreatedInTheirStartColumn(t *testing.T) {
	browserClient(t)
	session := mcpSessionAs(t, asUser(models.RoleAgent))
	create := func(args map[string]any) (string, error) {
		args["repo_id"] = seededRepoID(t)
		args["body"] = "Do the thing."
		made, err := callTool(t, session, "factory_create_task", args)
		if err != nil {
			return "", err
		}
		return str(t, made, "task", "id"), nil
	}
	column := func(id string) (string, *string) {
		var status string
		var readyBody *string
		require.NoError(t, testDB.QueryRow(context.Background(),
			`SELECT status, ready_body FROM factory.tasks WHERE id = $1`,
			taskKey(t, id)).Scan(&status, &readyBody))
		return status, readyBody
	}

	// A routine's finding is created in Ready, with its approved text.
	found, err := create(map[string]any{
		"title": "routine finding", "labels": []string{"routine-found"},
	})
	require.NoError(t, err)
	status, readyBody := column(found)
	assert.Equal(t, "ready", status)
	require.NotNil(t, readyBody)
	assert.Equal(t, "Do the thing.", *readyBody)
	var comment string
	require.NoError(t, testDB.QueryRow(context.Background(),
		`SELECT body FROM factory.task_comments WHERE task_id = $1`,
		taskKey(t, found)).Scan(&comment))
	assert.Contains(t, comment, "Created in Ready by agent:")

	asked, err := create(map[string]any{"title": "plain, asked ready", "status": "ready"})
	require.NoError(t, err)
	status, _ = column(asked)
	assert.Equal(t, "ready", status)

	kept, err := create(map[string]any{
		"title": "bug kept back", "labels": []string{"bug"}, "status": "backlog",
	})
	require.NoError(t, err)
	status, readyBody = column(kept)
	assert.Equal(t, "backlog", status)
	assert.Nil(t, readyBody)

	for _, s := range []string{"done", "testing", "in_progress", "nope"} {
		_, err = create(map[string]any{"title": "into " + s, "status": s})
		require.Error(t, err, s)
	}
}

func TestCreateTaskTakesOnlyBacklogOrReady(t *testing.T) {
	env, _ := accountsEnv(t)
	resp, err := env.tasks.CreateTask(context.Background(), &factoryv1.CreateTaskRequest{
		Title: "start " + uuid.NewString(), Body: "Do it.", RepoId: seededRepoID(t),
		Status: factoryv1.TaskStatus_TASK_STATUS_BACKLOG,
	})
	require.NoError(t, err)
	assert.Equal(t, factoryv1.TaskStatus_TASK_STATUS_BACKLOG, resp.Task.Status)
	_, err = env.tasks.CreateTask(context.Background(), &factoryv1.CreateTaskRequest{
		Title: "done " + uuid.NewString(), Body: "Do it.", RepoId: seededRepoID(t),
		Status: factoryv1.TaskStatus_TASK_STATUS_DONE,
	})
	require.Error(t, err)
}
