package todos_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	todosv1 "tools.xdoubleu.com/gen/todos/v1"
)

// TestTaskRPCs_InvalidID verifies every task RPC maps a malformed UUID to
// CodeNotFound instead of leaking an internal error.
func TestTaskRPCs_InvalidID(t *testing.T) {
	client := newTaskClient(t)
	const badID = "not-a-uuid"

	cases := map[string]func(ctx context.Context) error{
		"GetTask": func(ctx context.Context) error {
			_, err := client.GetTask(
				ctx, connect.NewRequest(&todosv1.GetTaskRequest{Id: badID}),
			)
			return err
		},
		"UpdateTask": func(ctx context.Context) error {
			_, err := client.UpdateTask(
				ctx, connect.NewRequest(&todosv1.UpdateTaskRequest{Id: badID}),
			)
			return err
		},
		"CompleteTask": func(ctx context.Context) error {
			_, err := client.CompleteTask(
				ctx, connect.NewRequest(&todosv1.CompleteTaskRequest{Id: badID}),
			)
			return err
		},
		"ReopenTask": func(ctx context.Context) error {
			_, err := client.ReopenTask(
				ctx, connect.NewRequest(&todosv1.ReopenTaskRequest{Id: badID}),
			)
			return err
		},
		"DeleteTask": func(ctx context.Context) error {
			_, err := client.DeleteTask(
				ctx, connect.NewRequest(&todosv1.DeleteTaskRequest{Id: badID}),
			)
			return err
		},
		"QuickUpdateTask": func(ctx context.Context) error {
			_, err := client.QuickUpdateTask(ctx, connect.NewRequest(
				&todosv1.QuickUpdateTaskRequest{Id: badID, Input: "x"},
			))
			return err
		},
		"MoveTaskSection": func(ctx context.Context) error {
			_, err := client.MoveTaskSection(ctx, connect.NewRequest(
				&todosv1.MoveTaskSectionRequest{TaskId: badID, SectionId: ""},
			))
			return err
		},
	}

	for name, call := range cases {
		t.Run(name, func(t *testing.T) {
			err := call(t.Context())
			require.Error(t, err)
			assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
		})
	}
}

// TestSubtaskRPCs_InvalidIDs verifies every subtask RPC maps malformed task and
// subtask UUIDs to CodeNotFound.
func TestSubtaskRPCs_InvalidIDs(t *testing.T) {
	client := newSubtaskClient(t)
	taskID := createTask(t, "subtask invalid-id task")
	const badID = "not-a-uuid"
	someID := uuid.New().String()

	cases := map[string]func(ctx context.Context) error{
		"CreateSubtask/badTask": func(ctx context.Context) error {
			_, err := client.CreateSubtask(ctx, connect.NewRequest(
				&todosv1.CreateSubtaskRequest{TaskId: badID, Input: "sub"},
			))
			return err
		},
		"ToggleSubtask/badTask": func(ctx context.Context) error {
			_, err := client.ToggleSubtask(ctx, connect.NewRequest(
				&todosv1.ToggleSubtaskRequest{TaskId: badID, SubtaskId: someID},
			))
			return err
		},
		"ToggleSubtask/badSubtask": func(ctx context.Context) error {
			_, err := client.ToggleSubtask(ctx, connect.NewRequest(
				&todosv1.ToggleSubtaskRequest{TaskId: taskID, SubtaskId: badID},
			))
			return err
		},
		"DeleteSubtask/badTask": func(ctx context.Context) error {
			_, err := client.DeleteSubtask(ctx, connect.NewRequest(
				&todosv1.DeleteSubtaskRequest{TaskId: badID, SubtaskId: someID},
			))
			return err
		},
		"DeleteSubtask/badSubtask": func(ctx context.Context) error {
			_, err := client.DeleteSubtask(ctx, connect.NewRequest(
				&todosv1.DeleteSubtaskRequest{TaskId: taskID, SubtaskId: badID},
			))
			return err
		},
		"ReorderSubtasks/badTask": func(ctx context.Context) error {
			_, err := client.ReorderSubtasks(ctx, connect.NewRequest(
				&todosv1.ReorderSubtasksRequest{TaskId: badID},
			))
			return err
		},
		"UpdateSubtask/badTask": func(ctx context.Context) error {
			_, err := client.UpdateSubtask(ctx, connect.NewRequest(
				&todosv1.UpdateSubtaskRequest{TaskId: badID, SubtaskId: someID},
			))
			return err
		},
		"UpdateSubtask/badSubtask": func(ctx context.Context) error {
			_, err := client.UpdateSubtask(ctx, connect.NewRequest(
				&todosv1.UpdateSubtaskRequest{TaskId: taskID, SubtaskId: badID},
			))
			return err
		},
	}

	for name, call := range cases {
		t.Run(name, func(t *testing.T) {
			err := call(t.Context())
			require.Error(t, err)
			assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
		})
	}
}

// TestSettingsRPCs_InvalidID verifies every ID-taking settings RPC maps a
// malformed UUID to CodeNotFound.
func TestSettingsRPCs_InvalidID(t *testing.T) {
	client := newSettingsClient(t)
	const badID = "not-a-uuid"

	cases := map[string]func(ctx context.Context) error{
		"DeleteURLPattern": func(ctx context.Context) error {
			_, err := client.DeleteURLPattern(ctx, connect.NewRequest(
				&todosv1.DeleteURLPatternRequest{Id: badID},
			))
			return err
		},
		"DeleteSection": func(ctx context.Context) error {
			_, err := client.DeleteSection(ctx, connect.NewRequest(
				&todosv1.DeleteSectionRequest{Id: badID},
			))
			return err
		},
		"UpdatePolicy": func(ctx context.Context) error {
			_, err := client.UpdatePolicy(ctx, connect.NewRequest(
				&todosv1.UpdatePolicyRequest{Id: badID, Text: "x"},
			))
			return err
		},
		"DeletePolicy": func(ctx context.Context) error {
			_, err := client.DeletePolicy(ctx, connect.NewRequest(
				&todosv1.DeletePolicyRequest{Id: badID},
			))
			return err
		},
		"DeleteWorkspace": func(ctx context.Context) error {
			_, err := client.DeleteWorkspace(ctx, connect.NewRequest(
				&todosv1.DeleteWorkspaceRequest{Id: badID},
			))
			return err
		},
	}

	for name, call := range cases {
		t.Run(name, func(t *testing.T) {
			err := call(t.Context())
			require.Error(t, err)
			assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
		})
	}
}

// TestMoveTaskSection_NonExistentSection documents that moving a task to a
// section that does not exist surfaces the FK violation as CodeInternal.
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
	require.Error(t, err)
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}
