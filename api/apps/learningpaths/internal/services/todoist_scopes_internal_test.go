package services

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/learningpaths/internal/models"
	"tools.xdoubleu.com/internal/database"
)

func TestSyncPath_StaleScopeIsNoOp(t *testing.T) {
	store := projectFixture(nil, nextItem("read"))
	svc, mock := newTestTodoistService(store)
	//nolint:exhaustruct //fixture connections
	svc.oauthRepo = &fakeConnections{connected: true, requestedScope: "task:add"}

	require.NoError(t, svc.SyncPath(t.Context(), "user-1", store.lp.ID))
	assert.Zero(t, mock.CreatedProjects)
	assert.Empty(t, store.taskIDWrites)
}

func TestStatus_ScopeCoverage(t *testing.T) {
	store := projectFixture(nil)
	svc, _ := newTestTodoistService(store)

	//nolint:exhaustruct //fixture connections
	svc.oauthRepo = &fakeConnections{connected: true, requestedScope: "task:add"}
	st, err := svc.Status(t.Context(), "user-1")
	require.NoError(t, err)
	assert.True(t, st.NeedsReconnect)

	//nolint:exhaustruct //fixture connections
	svc.oauthRepo = &fakeConnections{
		connected: true, requestedScope: "data:read_write,data:delete,project:delete",
	}
	st, err = svc.Status(t.Context(), "user-1")
	require.NoError(t, err)
	assert.False(t, st.NeedsReconnect)
}

func TestSyncState_ActiveModuleAndStrayTasks(t *testing.T) {
	projectID := "proj-1"
	done := nextItem("done")
	done.Completed = true
	stray := "stray-task"
	done.TodoistTaskID = &stray
	store := projectFixture(&projectID, done)
	store.modules = append(store.modules, models.Module{ //nolint:exhaustruct //fixture
		ID: uuid.New(), Title: "Week 2", SortOrder: 1,
		Items: []models.Item{nextItem("next")},
	})
	svc, _ := newTestTodoistService(store)

	state, err := svc.SyncState(t.Context(), "user-1", store.lp.ID)
	require.NoError(t, err)

	assert.Equal(t, "proj-1", state.ProjectID)
	assert.Equal(t, "Week 2", state.ActiveModule)
	require.Len(t, state.Items, 2)
	assert.Equal(t, "done", state.Items[0].Description)
	assert.Equal(t, "next", state.Items[1].Description)
}

func TestSyncState_Errors(t *testing.T) {
	store := projectFixture(nil)
	svc, _ := newTestTodoistService(store)

	_, err := svc.SyncState(t.Context(), "someone-else", store.lp.ID)
	assert.ErrorIs(t, err, database.ErrResourceNotFound)

	store.getModulesErr = errors.New("db")
	_, err = svc.SyncState(t.Context(), "user-1", store.lp.ID)
	assert.ErrorIs(t, err, store.getModulesErr)

	//nolint:exhaustruct //fixture connections
	svc.oauthRepo = &fakeConnections{statusErr: errors.New("status")}
	_, err = svc.SyncState(t.Context(), "user-1", store.lp.ID)
	assert.Error(t, err)
}
