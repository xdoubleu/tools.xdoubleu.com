package services

import (
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

// projectFixture is one path of items; projectID, when set, is the user's
// shared project.
func projectFixture(projectID *string, items ...models.Item) *fakeLearningPathsStore {
	//nolint:exhaustruct //fixture path
	lp := &models.LearningPath{ID: uuid.New(), UserID: "user-1", Title: "Architecture"}
	//nolint:exhaustruct //fixture store
	store := &fakeLearningPathsStore{
		lp:      lp,
		modules: []models.Module{{ID: uuid.New(), Items: items}},
	}
	if projectID != nil {
		store.userProjectID = *projectID
	}
	return store
}

// legacyProjectFixture is projectFixture whose path still has its own project.
func legacyProjectFixture(
	projectID string, items ...models.Item,
) *fakeLearningPathsStore {
	store := projectFixture(nil, items...)
	store.lp.TodoistProjectID = &projectID
	return store
}

func TestDeletePath_RemovesTasksAndLegacyProject(t *testing.T) {
	task := "t-1"
	item := nextItem("read")
	item.TodoistTaskID = &task
	store := legacyProjectFixture("proj-1", item)
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

func TestDelete_RemovesLegacyTodoistProject(t *testing.T) {
	store := legacyProjectFixture("proj-1", nextItem("read"))
	//nolint:exhaustruct //unset fields are the fixture defaults
	svc, mock := newConnectedService(store, &fakeBookLookup{})

	require.NoError(t, svc.Delete(t.Context(), store.lp.ID, store.lp.UserID))

	assert.True(t, store.deleted)
	assert.Equal(t, "proj-1", mock.LastDeletedProjectID)
}
