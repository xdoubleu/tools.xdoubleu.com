package factory_test

import (
	"context"
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	factoryv1 "tools.xdoubleu.com/gen/factory/v1"
)

func (e jobEnv) finished(
	t *testing.T,
	ids ...string,
) (*factoryv1.ListFinishedTasksResponse, error) {
	t.Helper()
	return e.runner.ListFinishedTasks(bearer(e.token),
		&factoryv1.ListFinishedTasksRequest{TaskIds: ids})
}

func TestFinishedTasksAreTheDoneAndTheGoneOnes(t *testing.T) {
	env := browserJobEnv(t)
	open := env.newTask(t, "still going")
	done := env.newTask(t, "finished")
	_, err := env.tasks.MoveTask(context.Background(), &factoryv1.MoveTaskRequest{
		Id: done.Id, Status: factoryv1.TaskStatus_TASK_STATUS_DONE,
	})
	require.NoError(t, err)
	gone := uuid.NewString()

	got, err := env.finished(t, open.Id, done.Id, gone)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{done.Id, gone}, got.TaskIds)

	none, err := env.finished(t)
	require.NoError(t, err)
	assert.Empty(t, none.TaskIds)
}

func TestFinishedTasksRefusesABadOrOversizedQuery(t *testing.T) {
	env := browserJobEnv(t)
	_, err := env.finished(t, "not-a-uuid")
	assert.Equal(t, connect.CodeInvalidArgument, code(err))

	ids := make([]string, 501)
	for i := range ids {
		ids[i] = uuid.NewString()
	}
	_, err = env.finished(t, ids...)
	assert.Equal(t, connect.CodeInvalidArgument, code(err))

	ids = ids[:500]
	_, err = env.finished(t, ids...)
	require.NoError(t, err, "500 is the most allowed")
}
