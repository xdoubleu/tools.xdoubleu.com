package services

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/database"
)

// pausedFixture is a path with one open item carrying a task in an existing
// project.
func pausedFixture(paused bool) *fakeLearningPathsStore {
	taskID := "task-7"
	item := nextItem("read ch1")
	item.TodoistTaskID = &taskID
	store := projectFixture(new("proj-1"), item)
	store.lp.Paused = paused
	return store
}

func TestSyncPath_PausedDeletesTasksAndKeepsProject(t *testing.T) {
	store := pausedFixture(true)
	svc, mock := newTestTodoistService(store)

	require.NoError(t, svc.SyncPath(t.Context(), "user-1", store.lp.ID))

	assert.Equal(t, "task-7", mock.LastDeletedID)
	assert.Equal(t, []string{""}, store.taskIDWrites)
	assert.Empty(t, mock.LastContent)
	assert.Empty(t, mock.LastDeletedProjectID)
	assert.Zero(t, mock.CreatedProjects)
}

func TestSyncPath_ResumedRecreatesActiveTasks(t *testing.T) {
	store := projectFixture(new("proj-1"), nextItem("read ch1"))
	svc, mock := newTestTodoistService(store)

	require.NoError(t, svc.SyncPath(t.Context(), "user-1", store.lp.ID))

	assert.Equal(t, "read ch1", mock.LastContent)
	assert.Equal(t, "proj-1", mock.LastProjectID)
}

func TestSyncState_PausedHasNoActiveModule(t *testing.T) {
	store := pausedFixture(true)
	store.modules[0].Title = "M1"
	svc, _ := newTestTodoistService(store)

	state, err := svc.SyncState(t.Context(), "user-1", store.lp.ID)
	require.NoError(t, err)

	assert.True(t, state.Paused)
	assert.Empty(t, state.ActiveModule)
	// The item still carrying a task is reported until the next sync.
	assert.Len(t, state.Items, 1)
}

func TestSetPaused_StoresFlag(t *testing.T) {
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{lp: newFixture()}
	svc := newTestService(store)

	require.NoError(t, svc.SetPaused(t.Context(), uuid.New(), "owner", true))
	require.NoError(t, svc.SetPaused(t.Context(), uuid.New(), "owner", false))
	assert.Equal(t, []bool{true, false}, store.pausedWrites)
}

func TestSetPaused_OtherUserNotFound(t *testing.T) {
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{lp: newFixture()}
	svc := newTestService(store)

	err := svc.SetPaused(t.Context(), uuid.New(), "other", true)
	assert.ErrorIs(t, err, database.ErrResourceNotFound)
	assert.Empty(t, store.pausedWrites)
}

func TestSetPaused_PropagatesErrors(t *testing.T) {
	dbErr := errors.New("db error")
	for name, store := range map[string]*fakeLearningPathsStore{
		"get": {getErr: dbErr},
		"set": {lp: newFixture(), setPausedErr: dbErr},
	} {
		t.Run(name, func(t *testing.T) {
			svc := newTestService(store)
			err := svc.SetPaused(t.Context(), uuid.New(), "owner", true)
			assert.ErrorIs(t, err, dbErr)
		})
	}
}

// TestSetPaused_PausingClearsTodoistTasks wires a connected Todoist so the
// pause reaches SyncPath.
func TestSetPaused_PausingClearsTodoistTasks(t *testing.T) {
	store := pausedFixture(true)
	todoistSvc, mock := newTestTodoistService(store)
	svc := newTestService(store)
	svc.todoist = todoistSvc

	require.NoError(t, svc.SetPaused(t.Context(), store.lp.ID, "user-1", true))
	assert.Equal(t, "task-7", mock.LastDeletedID)
}
