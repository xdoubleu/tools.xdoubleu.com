package todos_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	todosv1 "tools.xdoubleu.com/gen/todos/v1"
)

// TestListTasks_FiltersByWorkspaceID verifies the WorkspaceId filter returns
// tasks in that workspace and excludes tasks without one.
func TestListTasks_FiltersByWorkspaceID(t *testing.T) {
	wsID := getDefaultWorkspaceID(t)
	inWorkspace := createTaskInWorkspace(t, "workspace-filter task", wsID)
	outside := createTask(t, "no-workspace task")

	client := newTaskClient(t)
	resp, err := client.ListTasks(
		t.Context(),
		connect.NewRequest(&todosv1.ListTasksRequest{
			WorkspaceId: wsID,
		}),
	)
	require.NoError(t, err)

	ids := taskIDs(resp.Msg.Tasks)
	assert.Contains(t, ids, inWorkspace)
	assert.NotContains(t, ids, outside)
}

// TestListTasks_FiltersBySectionID verifies the SectionId filter returns only
// tasks placed in that section.
func TestListTasks_FiltersBySectionID(t *testing.T) {
	wsID := getDefaultWorkspaceID(t)
	client := newSettingsClient(t)

	_, err := client.CreateSection(
		t.Context(),
		connect.NewRequest(&todosv1.CreateSectionRequest{
			Name: "filter-section", WorkspaceId: wsID,
		}),
	)
	require.NoError(t, err)

	// GetSettings lists sections for the active workspace only.
	_, err = client.SetActiveWorkspace(
		t.Context(),
		connect.NewRequest(&todosv1.SetActiveWorkspaceRequest{WorkspaceId: wsID}),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = client.SetActiveWorkspace(
			context.Background(),
			connect.NewRequest(&todosv1.SetActiveWorkspaceRequest{}),
		)
	})

	settingsResp, err := client.GetSettings(
		t.Context(),
		connect.NewRequest(&todosv1.GetSettingsRequest{}),
	)
	require.NoError(t, err)

	var sectionID string
	for _, s := range settingsResp.Msg.Sections {
		if s.Name == "filter-section" {
			sectionID = s.Id
		}
	}
	require.NotEmpty(t, sectionID)

	taskClient := newTaskClient(t)
	inSection := createTask(t, "in-section task")
	_, err = taskClient.MoveTaskSection(
		t.Context(),
		connect.NewRequest(&todosv1.MoveTaskSectionRequest{
			TaskId: inSection, SectionId: sectionID,
		}),
	)
	require.NoError(t, err)
	outside := createTask(t, "outside-section task")

	resp, err := taskClient.ListTasks(
		t.Context(),
		connect.NewRequest(&todosv1.ListTasksRequest{SectionId: sectionID}),
	)
	require.NoError(t, err)

	ids := taskIDs(resp.Msg.Tasks)
	assert.Contains(t, ids, inSection)
	assert.NotContains(t, ids, outside)
}

// TestGetTask_WithLinks verifies task links are returned on the task proto.
func TestGetTask_WithLinks(t *testing.T) {
	taskID := createTask(t, "linked task")

	_, err := testDB.Exec(t.Context(), `
		INSERT INTO todos.task_links (task_id, url, label)
		VALUES ($1::uuid, 'https://example.com', 'example')`,
		taskID,
	)
	require.NoError(t, err)

	client := newTaskClient(t)
	resp, err := client.GetTask(
		t.Context(),
		connect.NewRequest(&todosv1.GetTaskRequest{Id: taskID}),
	)
	require.NoError(t, err)
	require.Len(t, resp.Msg.Task.Links, 1)
	assert.Equal(t, "https://example.com", resp.Msg.Task.Links[0].Url)
	assert.Equal(t, "example", resp.Msg.Task.Links[0].Label)
}

// TestAddSubtask_Nested verifies a subtask created with ParentSubtaskId is
// returned nested under its parent's Children.
func TestAddSubtask_Nested(t *testing.T) {
	taskID := createTask(t, "nested subtask task")
	client := newSubtaskClient(t)

	parentResp, err := client.CreateSubtask(
		t.Context(),
		connect.NewRequest(&todosv1.CreateSubtaskRequest{
			TaskId: taskID, Input: "parent sub",
		}),
	)
	require.NoError(t, err)
	parentSubID := parentResp.Msg.Subtask.Id

	childResp, err := client.CreateSubtask(
		t.Context(),
		connect.NewRequest(&todosv1.CreateSubtaskRequest{
			TaskId:          taskID,
			Input:           "child sub",
			ParentSubtaskId: parentSubID,
		}),
	)
	require.NoError(t, err)
	childID := childResp.Msg.Subtask.Id

	taskClient := newTaskClient(t)
	resp, err := taskClient.GetTask(
		t.Context(),
		connect.NewRequest(&todosv1.GetTaskRequest{Id: taskID}),
	)
	require.NoError(t, err)

	var parent *todosv1.Subtask
	for _, s := range resp.Msg.Task.Subtasks {
		if s.Id == parentSubID {
			parent = s
		}
	}
	require.NotNil(t, parent, "parent subtask should be top-level")
	require.Len(t, parent.Children, 1)
	assert.Equal(t, childID, parent.Children[0].Id)
}

// TestGetTask_WithOptionalFields verifies due_date and deadline round-trip
// through the task proto.
func TestGetTask_WithOptionalFields(t *testing.T) {
	var id string
	err := testDB.QueryRow(t.Context(), `
		INSERT INTO todos.tasks (owner_user_id, title, due_date, deadline)
		VALUES ($1, 'rich task', '2026-12-31', '2026-12-31 23:59:00+00')
		RETURNING id::text`,
		userID,
	).Scan(&id)
	require.NoError(t, err)

	client := newTaskClient(t)
	resp, err := client.GetTask(
		t.Context(),
		connect.NewRequest(&todosv1.GetTaskRequest{Id: id}),
	)
	require.NoError(t, err)
	assert.Equal(t, "2026-12-31", resp.Msg.Task.DueDate)
	assert.NotEmpty(t, resp.Msg.Task.Deadline)
}

// createTaskInWorkspace inserts a task assigned to the given workspace.
func createTaskInWorkspace(t *testing.T, title, workspaceID string) string {
	t.Helper()
	var id string
	err := testDB.QueryRow(t.Context(), `
		INSERT INTO todos.tasks (owner_user_id, title, workspace_id)
		VALUES ($1, $2, $3::uuid)
		RETURNING id::text`,
		userID, title, workspaceID,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

// taskIDs extracts the IDs from a list of task protos.
func taskIDs(tasks []*todosv1.Task) []string {
	ids := make([]string, 0, len(tasks))
	for _, task := range tasks {
		ids = append(ids, task.Id)
	}
	return ids
}
