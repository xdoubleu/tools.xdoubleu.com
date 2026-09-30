package services

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/learningpaths/internal/models"
)

func scheduledItem(description, due string) models.Item {
	it := nextItem(description)
	it.Due = due
	return it
}

func projectFixture(projectID *string, items ...models.Item) *fakeLearningPathsStore {
	//nolint:exhaustruct //fixture path
	lp := &models.LearningPath{
		ID: uuid.New(), UserID: "user-1", Title: "Architecture",
		TodoistProjectID: projectID,
	}
	//nolint:exhaustruct //fixture store
	return &fakeLearningPathsStore{
		lp:      lp,
		modules: []models.Module{{ID: uuid.New(), Items: items}},
	}
}

func TestSyncPath_FirstSyncCreatesProjectAndScheduledTasks(t *testing.T) {
	store := projectFixture(nil, scheduledItem("read the book", "every day at 20:00"))
	svc, mock := newTestTodoistService(store)
	mock.ProjectID = "proj-new"

	require.NoError(t, svc.SyncPath(t.Context(), "user-1", store.lp.ID))

	assert.Equal(t, 1, mock.CreatedProjects)
	assert.Equal(t, []string{"proj-new"}, store.projectIDWrites)
	assert.Equal(t, "read the book", mock.LastContent)
	assert.Equal(t, "every day at 20:00", mock.LastDueString)
	assert.Equal(t, "proj-new", mock.LastProjectID)
}

func TestSyncPath_DueFromTypeScheduleUnlessItemOverrides(t *testing.T) {
	read := nextItem("read the book")
	read.Type = "Read"
	checkpoint := scheduledItem("pass the quiz", "every sat at 10:00")
	checkpoint.Type = "checkpoint"
	store := projectFixture(nil, read, checkpoint)
	store.lp.ReminderSchedules = map[string]string{
		"read": "every day at 20:00", "checkpoint": "every sun at 18:00",
	}
	svc, mock := newTestTodoistService(store)
	mock.ProjectID = "proj-new"

	require.NoError(t, svc.SyncPath(t.Context(), "user-1", store.lp.ID))

	assert.Equal(t,
		[]string{"every day at 20:00", "every sat at 10:00"}, mock.DueStrings,
	)
}

func TestSyncPath_ExistingProjectReused(t *testing.T) {
	projectID := "proj-1"
	store := projectFixture(&projectID, scheduledItem("do it", "every wed at 19:00"))
	svc, mock := newTestTodoistService(store)

	require.NoError(t, svc.SyncPath(t.Context(), "user-1", store.lp.ID))

	assert.Zero(t, mock.CreatedProjects)
	assert.Equal(t, "proj-1", mock.LastProjectID)
}

// TestSyncPath_RecreatedProjectReplacesTasks covers both a project deleted in
// Todoist and tasks created in the Inbox before paths had projects.
func TestSyncPath_RecreatedProjectReplacesTasks(t *testing.T) {
	projectID := "proj-gone"
	oldTask := "inbox-task"
	item := nextItem("read the book")
	item.TodoistTaskID = &oldTask
	store := projectFixture(&projectID, item)
	svc, mock := newTestTodoistService(store)
	mock.ProjectMissing = true
	mock.ProjectID = "proj-new"

	require.NoError(t, svc.SyncPath(t.Context(), "user-1", store.lp.ID))

	assert.Equal(t, "inbox-task", mock.LastDeletedID)
	assert.Equal(t, "proj-new", mock.LastProjectID)
	assert.Equal(t, []string{"", "task-123"}, store.taskIDWrites)
}

func TestSyncPath_PropagatesProjectError(t *testing.T) {
	projectID := "proj-1"
	store := projectFixture(&projectID, nextItem("read"))
	svc, mock := newTestTodoistService(store)
	mock.Err = errors.New("todoist down")

	err := svc.SyncPath(t.Context(), "user-1", store.lp.ID)
	assert.ErrorIs(t, err, mock.Err)
	assert.Empty(t, store.taskIDWrites)
}

func TestDeletePath_RemovesTasksAndProject(t *testing.T) {
	projectID := "proj-1"
	task := "t-1"
	item := nextItem("read")
	item.TodoistTaskID = &task
	store := projectFixture(&projectID, item)
	svc, mock := newTestTodoistService(store)

	require.NoError(t, svc.DeletePath(t.Context(), "user-1", store.lp, store.modules))

	assert.Equal(t, "t-1", mock.LastDeletedID)
	assert.Equal(t, "proj-1", mock.LastDeletedProjectID)
}

func TestDeletePath_WithoutProjectOnlyRemovesTasks(t *testing.T) {
	task := "t-1"
	item := nextItem("read")
	item.TodoistTaskID = &task
	store := projectFixture(nil, item)
	svc, mock := newTestTodoistService(store)

	require.NoError(t, svc.DeletePath(t.Context(), "user-1", store.lp, store.modules))

	assert.Equal(t, "t-1", mock.LastDeletedID)
	assert.Empty(t, mock.LastDeletedProjectID)
}

func TestDeletePath_NotConnectedIsNoOp(t *testing.T) {
	projectID := "proj-1"
	store := projectFixture(&projectID, nextItem("read"))
	svc, mock := newTestTodoistService(store)
	//nolint:exhaustruct //fixture connections
	svc.oauthRepo = &fakeConnections{connected: false}

	require.NoError(t, svc.DeletePath(t.Context(), "user-1", store.lp, store.modules))
	assert.Empty(t, mock.LastDeletedProjectID)
}

func TestDelete_RemovesTodoistProject(t *testing.T) {
	projectID := "proj-1"
	store := projectFixture(&projectID, nextItem("read"))
	//nolint:exhaustruct //unset fields are the fixture defaults
	svc, mock := newConnectedService(store, &fakeBookLookup{})

	require.NoError(t, svc.Delete(t.Context(), store.lp.ID, store.lp.UserID))

	assert.True(t, store.deleted)
	assert.Equal(t, "proj-1", mock.LastDeletedProjectID)
}
