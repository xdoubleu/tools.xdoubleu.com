package todos_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	todosv1 "tools.xdoubleu.com/gen/todos/v1"
)

// ── Task invalid-ID paths ─────────────────────────────────────────────────────

func TestGetTask_InvalidID(t *testing.T) {
	client := newTaskClient(t)
	_, err := client.GetTask(
		t.Context(),
		connect.NewRequest(&todosv1.GetTaskRequest{Id: "not-a-uuid"}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestUpdateTask_InvalidID(t *testing.T) {
	client := newTaskClient(t)
	_, err := client.UpdateTask(
		t.Context(),
		connect.NewRequest(&todosv1.UpdateTaskRequest{Id: "not-a-uuid"}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestCompleteTask_InvalidID(t *testing.T) {
	client := newTaskClient(t)
	_, err := client.CompleteTask(
		t.Context(),
		connect.NewRequest(&todosv1.CompleteTaskRequest{Id: "not-a-uuid"}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestReopenTask_InvalidID(t *testing.T) {
	client := newTaskClient(t)
	_, err := client.ReopenTask(
		t.Context(),
		connect.NewRequest(&todosv1.ReopenTaskRequest{Id: "not-a-uuid"}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestDeleteTask_InvalidID(t *testing.T) {
	client := newTaskClient(t)
	_, err := client.DeleteTask(
		t.Context(),
		connect.NewRequest(&todosv1.DeleteTaskRequest{Id: "not-a-uuid"}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestQuickUpdateTask_InvalidID(t *testing.T) {
	client := newTaskClient(t)
	_, err := client.QuickUpdateTask(
		t.Context(),
		connect.NewRequest(
			&todosv1.QuickUpdateTaskRequest{Id: "not-a-uuid", Input: "x"},
		),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestMoveTaskSection_InvalidTaskID(t *testing.T) {
	client := newTaskClient(t)
	_, err := client.MoveTaskSection(
		t.Context(),
		connect.NewRequest(&todosv1.MoveTaskSectionRequest{
			TaskId: "not-a-uuid", SectionId: "",
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestMoveTaskSection_NonExistentSection(t *testing.T) {
	taskID := createTask(t, "MoveSection task")
	client := newTaskClient(t)
	_, err := client.MoveTaskSection(
		t.Context(),
		connect.NewRequest(&todosv1.MoveTaskSectionRequest{
			TaskId:    taskID,
			SectionId: uuid.New().String(),
		}),
	)
	// Non-existent section triggers a DB FK error → internal code.
	require.Error(t, err)
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}

// ── Subtask invalid-ID paths ──────────────────────────────────────────────────

func TestAddSubtask_InvalidTaskID(t *testing.T) {
	client := newSubtaskClient(t)
	_, err := client.AddSubtask(
		t.Context(),
		connect.NewRequest(&todosv1.AddSubtaskRequest{
			TaskId: "not-a-uuid", Input: "sub",
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestToggleSubtask_InvalidTaskID(t *testing.T) {
	client := newSubtaskClient(t)
	_, err := client.ToggleSubtask(
		t.Context(),
		connect.NewRequest(&todosv1.ToggleSubtaskRequest{
			TaskId: "not-a-uuid", SubtaskId: uuid.New().String(),
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestToggleSubtask_InvalidSubtaskID(t *testing.T) {
	taskID := createTask(t, "ToggleSubtask task")
	client := newSubtaskClient(t)
	_, err := client.ToggleSubtask(
		t.Context(),
		connect.NewRequest(&todosv1.ToggleSubtaskRequest{
			TaskId: taskID, SubtaskId: "not-a-uuid",
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestDeleteSubtask_InvalidTaskID(t *testing.T) {
	client := newSubtaskClient(t)
	_, err := client.DeleteSubtask(
		t.Context(),
		connect.NewRequest(&todosv1.DeleteSubtaskRequest{
			TaskId: "not-a-uuid", SubtaskId: uuid.New().String(),
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestDeleteSubtask_InvalidSubtaskID(t *testing.T) {
	taskID := createTask(t, "DeleteSubtask task")
	client := newSubtaskClient(t)
	_, err := client.DeleteSubtask(
		t.Context(),
		connect.NewRequest(&todosv1.DeleteSubtaskRequest{
			TaskId: taskID, SubtaskId: "not-a-uuid",
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestReorderSubtasks_InvalidTaskID(t *testing.T) {
	client := newSubtaskClient(t)
	_, err := client.ReorderSubtasks(
		t.Context(),
		connect.NewRequest(&todosv1.ReorderSubtasksRequest{
			TaskId: "not-a-uuid",
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestUpdateSubtask_InvalidTaskID(t *testing.T) {
	client := newSubtaskClient(t)
	_, err := client.UpdateSubtask(
		t.Context(),
		connect.NewRequest(&todosv1.UpdateSubtaskRequest{
			TaskId: "not-a-uuid", SubtaskId: uuid.New().String(),
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestUpdateSubtask_InvalidSubtaskID(t *testing.T) {
	taskID := createTask(t, "UpdateSubtask task")
	client := newSubtaskClient(t)
	_, err := client.UpdateSubtask(
		t.Context(),
		connect.NewRequest(&todosv1.UpdateSubtaskRequest{
			TaskId: taskID, SubtaskId: "not-a-uuid",
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}
