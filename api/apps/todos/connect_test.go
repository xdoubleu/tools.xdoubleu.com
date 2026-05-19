package todos_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	todosv1 "tools.xdoubleu.com/gen/todos/v1"
	"tools.xdoubleu.com/gen/todos/v1/todosv1connect"
)

func newTaskClient(t *testing.T) todosv1connect.TaskServiceClient {
	t.Helper()
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)
	return todosv1connect.NewTaskServiceClient(http.DefaultClient, ts.URL)
}

func newSubtaskClient(t *testing.T) todosv1connect.SubtaskServiceClient {
	t.Helper()
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)
	return todosv1connect.NewSubtaskServiceClient(http.DefaultClient, ts.URL)
}

func newSettingsClient(t *testing.T) todosv1connect.SettingsServiceClient {
	t.Helper()
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)
	return todosv1connect.NewSettingsServiceClient(http.DefaultClient, ts.URL)
}

// ── ListTasks ─────────────────────────────────────────────────────────────────

func TestListTasks_Open(t *testing.T) {
	client := newTaskClient(t)
	_, err := client.ListTasks(
		t.Context(),
		connect.NewRequest(&todosv1.ListTasksRequest{Status: ""}),
	)
	require.NoError(t, err)
}

func TestListTasks_Done(t *testing.T) {
	client := newTaskClient(t)
	_, err := client.ListTasks(
		t.Context(),
		connect.NewRequest(&todosv1.ListTasksRequest{Status: "done"}),
	)
	require.NoError(t, err)
}

func TestListTasks_Archived(t *testing.T) {
	client := newTaskClient(t)
	_, err := client.ListTasks(
		t.Context(),
		connect.NewRequest(&todosv1.ListTasksRequest{Status: "archived"}),
	)
	require.NoError(t, err)
}

// ── GetTask ───────────────────────────────────────────────────────────────────

func TestGetTask_Found(t *testing.T) {
	id := createTask(t, "GetTask test")
	client := newTaskClient(t)

	resp, err := client.GetTask(
		t.Context(),
		connect.NewRequest(&todosv1.GetTaskRequest{Id: id}),
	)
	require.NoError(t, err)
	assert.Equal(t, id, resp.Msg.Task.Id)
}

func TestGetTask_NotFound(t *testing.T) {
	client := newTaskClient(t)
	_, err := client.GetTask(
		t.Context(),
		connect.NewRequest(
			&todosv1.GetTaskRequest{Id: "00000000-0000-0000-0000-000000000000"},
		),
	)
	require.Error(t, err)
}

// ── CreateTask ────────────────────────────────────────────────────────────────

func TestCreateTask(t *testing.T) {
	client := newTaskClient(t)
	resp, err := client.CreateTask(
		t.Context(),
		connect.NewRequest(&todosv1.CreateTaskRequest{Title: "Created via RPC"}),
	)
	require.NoError(t, err)
	assert.Equal(t, "Created via RPC", resp.Msg.Task.Title)
}

// ── UpdateTask ────────────────────────────────────────────────────────────────

func TestUpdateTask(t *testing.T) {
	id := createTask(t, "UpdateTask original")
	client := newTaskClient(t)

	resp, err := client.UpdateTask(
		t.Context(),
		connect.NewRequest(&todosv1.UpdateTaskRequest{
			Id:    id,
			Title: "UpdateTask updated",
		}),
	)
	require.NoError(t, err)
	assert.Equal(t, "UpdateTask updated", resp.Msg.Task.Title)
}

// ── CompleteTask / ReopenTask ─────────────────────────────────────────────────

func TestCompleteTask(t *testing.T) {
	id := createTask(t, "CompleteTask test")
	client := newTaskClient(t)

	_, err := client.CompleteTask(
		t.Context(),
		connect.NewRequest(&todosv1.CompleteTaskRequest{Id: id}),
	)
	require.NoError(t, err)
}

func TestReopenTask(t *testing.T) {
	id := createTask(t, "ReopenTask test")
	client := newTaskClient(t)

	_, err := client.CompleteTask(
		t.Context(),
		connect.NewRequest(&todosv1.CompleteTaskRequest{Id: id}),
	)
	require.NoError(t, err)

	_, err = client.ReopenTask(
		t.Context(),
		connect.NewRequest(&todosv1.ReopenTaskRequest{Id: id}),
	)
	require.NoError(t, err)
}

// ── DeleteTask ────────────────────────────────────────────────────────────────

func TestDeleteTask(t *testing.T) {
	id := createTask(t, "DeleteTask test")
	client := newTaskClient(t)

	_, err := client.DeleteTask(
		t.Context(),
		connect.NewRequest(&todosv1.DeleteTaskRequest{Id: id}),
	)
	require.NoError(t, err)
}

// ── ReorderTasks ──────────────────────────────────────────────────────────────

func TestReorderTasks(t *testing.T) {
	id1 := createTask(t, "Reorder 1")
	id2 := createTask(t, "Reorder 2")
	client := newTaskClient(t)

	_, err := client.ReorderTasks(
		t.Context(),
		connect.NewRequest(&todosv1.ReorderTasksRequest{Ids: []string{id2, id1}}),
	)
	require.NoError(t, err)
}

// ── SearchTasks ───────────────────────────────────────────────────────────────

func TestSearchTasks_Empty(t *testing.T) {
	client := newTaskClient(t)
	resp, err := client.SearchTasks(
		t.Context(),
		connect.NewRequest(&todosv1.SearchTasksRequest{Query: ""}),
	)
	require.NoError(t, err)
	assert.NotNil(t, resp.Msg)
}

func TestSearchTasks_WithQuery(t *testing.T) {
	unique := "UniqSearchRPC99887766"
	createTask(t, unique)
	client := newTaskClient(t)

	resp, err := client.SearchTasks(
		t.Context(),
		connect.NewRequest(&todosv1.SearchTasksRequest{Query: unique}),
	)
	require.NoError(t, err)
	found := false
	for _, task := range resp.Msg.Open {
		if task.Title == unique {
			found = true
		}
	}
	assert.True(t, found)
}

// ── QuickAddTask ──────────────────────────────────────────────────────────────

func TestQuickAddTask(t *testing.T) {
	client := newTaskClient(t)
	resp, err := client.QuickAddTask(
		t.Context(),
		connect.NewRequest(&todosv1.QuickAddTaskRequest{Input: "Quick add title"}),
	)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Msg.Task.Id)
}

// ── QuickUpdateTask ───────────────────────────────────────────────────────────

func TestQuickUpdateTask(t *testing.T) {
	id := createTask(t, "QuickUpdate original")
	client := newTaskClient(t)

	_, err := client.QuickUpdateTask(
		t.Context(),
		connect.NewRequest(&todosv1.QuickUpdateTaskRequest{
			Id:    id,
			Input: "QuickUpdate updated",
		}),
	)
	require.NoError(t, err)
}

// ── Subtasks ──────────────────────────────────────────────────────────────────

func TestAddSubtask(t *testing.T) {
	taskID := createTask(t, "AddSubtask parent")
	client := newSubtaskClient(t)

	resp, err := client.AddSubtask(
		t.Context(),
		connect.NewRequest(&todosv1.AddSubtaskRequest{
			TaskId: taskID,
			Input:  "subtask 1",
		}),
	)
	require.NoError(t, err)
	assert.Equal(t, "subtask 1", resp.Msg.Subtask.Title)
}

func TestUpdateSubtask(t *testing.T) {
	taskID := createTask(t, "UpdateSubtask parent")
	client := newSubtaskClient(t)

	addResp, err := client.AddSubtask(
		t.Context(),
		connect.NewRequest(&todosv1.AddSubtaskRequest{
			TaskId: taskID,
			Input:  "original subtask",
		}),
	)
	require.NoError(t, err)
	sid := addResp.Msg.Subtask.Id

	resp, err := client.UpdateSubtask(
		t.Context(),
		connect.NewRequest(&todosv1.UpdateSubtaskRequest{
			TaskId:    taskID,
			SubtaskId: sid,
			Title:     "updated subtask",
		}),
	)
	require.NoError(t, err)
	assert.Equal(t, "updated subtask", resp.Msg.Subtask.Title)
}

func TestToggleSubtask(t *testing.T) {
	taskID := createTask(t, "ToggleSubtask parent")
	client := newSubtaskClient(t)

	addResp, err := client.AddSubtask(
		t.Context(),
		connect.NewRequest(&todosv1.AddSubtaskRequest{
			TaskId: taskID,
			Input:  "toggle me",
		}),
	)
	require.NoError(t, err)

	_, err = client.ToggleSubtask(
		t.Context(),
		connect.NewRequest(&todosv1.ToggleSubtaskRequest{
			TaskId:    taskID,
			SubtaskId: addResp.Msg.Subtask.Id,
		}),
	)
	require.NoError(t, err)
}

func TestDeleteSubtask(t *testing.T) {
	taskID := createTask(t, "DeleteSubtask parent")
	client := newSubtaskClient(t)

	addResp, err := client.AddSubtask(
		t.Context(),
		connect.NewRequest(&todosv1.AddSubtaskRequest{
			TaskId: taskID,
			Input:  "delete me",
		}),
	)
	require.NoError(t, err)

	_, err = client.DeleteSubtask(
		t.Context(),
		connect.NewRequest(&todosv1.DeleteSubtaskRequest{
			TaskId:    taskID,
			SubtaskId: addResp.Msg.Subtask.Id,
		}),
	)
	require.NoError(t, err)
}

func TestReorderSubtasks(t *testing.T) {
	taskID := createTask(t, "ReorderSubtasks parent")
	client := newSubtaskClient(t)

	r1, err := client.AddSubtask(t.Context(),
		connect.NewRequest(&todosv1.AddSubtaskRequest{TaskId: taskID, Input: "s1"}))
	require.NoError(t, err)
	r2, err := client.AddSubtask(t.Context(),
		connect.NewRequest(&todosv1.AddSubtaskRequest{TaskId: taskID, Input: "s2"}))
	require.NoError(t, err)

	_, err = client.ReorderSubtasks(
		t.Context(),
		connect.NewRequest(&todosv1.ReorderSubtasksRequest{
			TaskId: taskID,
			Ids:    []string{r2.Msg.Subtask.Id, r1.Msg.Subtask.Id},
		}),
	)
	require.NoError(t, err)
}

// ── MoveTaskSection ───────────────────────────────────────────────────────────

func TestMoveTaskSection_ToNoSection(t *testing.T) {
	id := createTask(t, "MoveSection test")
	client := newTaskClient(t)

	_, err := client.MoveTaskSection(
		t.Context(),
		connect.NewRequest(&todosv1.MoveTaskSectionRequest{
			TaskId:    id,
			SectionId: "",
		}),
	)
	require.NoError(t, err)
}

// ── GetSettings ───────────────────────────────────────────────────────────────

func TestGetSettings(t *testing.T) {
	client := newSettingsClient(t)
	resp, err := client.GetSettings(
		t.Context(),
		connect.NewRequest(&todosv1.GetSettingsRequest{}),
	)
	require.NoError(t, err)
	assert.NotNil(t, resp.Msg)
}

// ── Settings mutations ────────────────────────────────────────────────────────

func TestUpdateArchiveSettings(t *testing.T) {
	client := newSettingsClient(t)
	_, err := client.UpdateArchiveSettings(
		t.Context(),
		connect.NewRequest(
			&todosv1.UpdateArchiveSettingsRequest{ArchiveAfterHours: 48},
		),
	)
	require.NoError(t, err)
}

func TestAddSection_RemoveSection(t *testing.T) {
	client := newSettingsClient(t)

	addResp, err := client.AddSection(
		t.Context(),
		connect.NewRequest(&todosv1.AddSectionRequest{Name: "Test Section"}),
	)
	require.NoError(t, err)
	_ = addResp

	settings, err := client.GetSettings(
		t.Context(),
		connect.NewRequest(&todosv1.GetSettingsRequest{}),
	)
	require.NoError(t, err)

	var sectionID string
	for _, s := range settings.Msg.Sections {
		if s.Name == "Test Section" {
			sectionID = s.Id
		}
	}
	require.NotEmpty(t, sectionID)

	_, err = client.RemoveSection(
		t.Context(),
		connect.NewRequest(&todosv1.RemoveSectionRequest{Id: sectionID}),
	)
	require.NoError(t, err)
}

func TestAddWorkspace_DeleteWorkspace(t *testing.T) {
	client := newSettingsClient(t)

	_, err := client.AddWorkspace(
		t.Context(),
		connect.NewRequest(&todosv1.AddWorkspaceRequest{Name: "Test WS"}),
	)
	require.NoError(t, err)

	settings, err := client.GetSettings(
		t.Context(),
		connect.NewRequest(&todosv1.GetSettingsRequest{}),
	)
	require.NoError(t, err)

	var wsID string
	for _, w := range settings.Msg.Workspaces {
		if w.Name == "Test WS" {
			wsID = w.Id
		}
	}
	require.NotEmpty(t, wsID)

	_, err = client.DeleteWorkspace(
		t.Context(),
		connect.NewRequest(&todosv1.DeleteWorkspaceRequest{Id: wsID}),
	)
	require.NoError(t, err)
}

func TestAddPolicy_UpdatePolicy_RemovePolicy(t *testing.T) {
	client := newSettingsClient(t)

	_, err := client.AddPolicy(
		t.Context(),
		connect.NewRequest(&todosv1.AddPolicyRequest{
			Text:               "Test policy",
			ReappearAfterHours: 24,
		}),
	)
	require.NoError(t, err)

	settings, err := client.GetSettings(
		t.Context(),
		connect.NewRequest(&todosv1.GetSettingsRequest{}),
	)
	require.NoError(t, err)

	var policyID string
	for _, p := range settings.Msg.Policies {
		if p.Text == "Test policy" {
			policyID = p.Id
		}
	}
	require.NotEmpty(t, policyID)

	_, err = client.UpdatePolicy(
		t.Context(),
		connect.NewRequest(&todosv1.UpdatePolicyRequest{
			Id:                 policyID,
			Text:               "Updated policy",
			ReappearAfterHours: 48,
		}),
	)
	require.NoError(t, err)

	_, err = client.RemovePolicy(
		t.Context(),
		connect.NewRequest(&todosv1.RemovePolicyRequest{Id: policyID}),
	)
	require.NoError(t, err)
}

func TestUpdateHideShortcutHints(t *testing.T) {
	client := newSettingsClient(t)
	_, err := client.UpdateHideShortcutHints(
		t.Context(),
		connect.NewRequest(&todosv1.UpdateHideShortcutHintsRequest{Hide: true}),
	)
	require.NoError(t, err)
}

func TestSetActiveWorkspace(t *testing.T) {
	client := newSettingsClient(t)
	_, err := client.SetActiveWorkspace(
		t.Context(),
		connect.NewRequest(&todosv1.SetActiveWorkspaceRequest{WorkspaceId: ""}),
	)
	require.NoError(t, err)
}
