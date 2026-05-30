package todos_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	todosv1 "tools.xdoubleu.com/gen/todos/v1"
)

// TestListTasks_WithWorkspaceID covers the WorkspaceId branch in ListTasks.
func TestListTasks_WithWorkspaceID(t *testing.T) {
	wsID := getDefaultWorkspaceID(t)
	client := newTaskClient(t)
	_, err := client.ListTasks(
		t.Context(),
		connect.NewRequest(&todosv1.ListTasksRequest{
			WorkspaceId: wsID,
		}),
	)
	require.NoError(t, err)
}

// TestListTasks_WithSectionID covers the SectionId branch in ListTasks.
func TestListTasks_WithSectionID(t *testing.T) {
	wsID := getDefaultWorkspaceID(t)
	client := newSettingsClient(t)

	_, err := client.AddSection(
		t.Context(),
		connect.NewRequest(&todosv1.AddSectionRequest{
			Name: "coverage-section", WorkspaceId: wsID,
		}),
	)
	require.NoError(t, err)

	settingsResp, err := client.GetSettings(
		t.Context(),
		connect.NewRequest(&todosv1.GetSettingsRequest{}),
	)
	require.NoError(t, err)

	var sectionID string
	for _, s := range settingsResp.Msg.Sections {
		if s.Name == "coverage-section" {
			sectionID = s.Id
		}
	}
	require.NotEmpty(t, sectionID)

	taskClient := newTaskClient(t)
	_, err = taskClient.ListTasks(
		t.Context(),
		connect.NewRequest(&todosv1.ListTasksRequest{SectionId: sectionID}),
	)
	require.NoError(t, err)
}

// TestGetTask_WithLinks covers the task links loop body in protoTask.
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
	assert.NotEmpty(t, resp.Msg.Task.Links)
}

// TestAddSubtask_Nested covers the children loop body in protoSubtask.
func TestAddSubtask_Nested(t *testing.T) {
	taskID := createTask(t, "nested subtask task")
	client := newSubtaskClient(t)

	parentResp, err := client.AddSubtask(
		t.Context(),
		connect.NewRequest(&todosv1.AddSubtaskRequest{
			TaskId: taskID, Input: "parent sub",
		}),
	)
	require.NoError(t, err)
	parentSubID := parentResp.Msg.Subtask.Id

	_, err = client.AddSubtask(
		t.Context(),
		connect.NewRequest(&todosv1.AddSubtaskRequest{
			TaskId:          taskID,
			Input:           "child sub",
			ParentSubtaskId: parentSubID,
		}),
	)
	require.NoError(t, err)

	taskClient := newTaskClient(t)
	resp, err := taskClient.GetTask(
		t.Context(),
		connect.NewRequest(&todosv1.GetTaskRequest{Id: taskID}),
	)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Msg.Task.Subtasks)
}

// TestGetTask_WithOptionalFields covers non-nil branches of datePtrToStr,
// timePtrToRFC3339, and uuidPtrToStr in protoTask by inserting a task
// directly with due_date, deadline, and workspace_id set.
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
	assert.NotEmpty(t, resp.Msg.Task.DueDate)
	assert.NotEmpty(t, resp.Msg.Task.Deadline)
}
