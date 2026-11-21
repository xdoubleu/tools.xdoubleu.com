package services

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/learningpaths/internal/models"
)

func typedItem(itemType, description string) models.Item {
	it := nextItem(description)
	it.Type = itemType
	return it
}

func withTask(it models.Item, taskID string) models.Item {
	it.TodoistTaskID = &taskID
	return it
}

func done(it models.Item) models.Item {
	it.Completed = true
	return it
}

// multiPathStore holds one path per module list, created in the given order,
// sharing project "proj-1".
func multiPathStore(pathModules ...[]models.Module) *fakeLearningPathsStore {
	//nolint:exhaustruct //fixture store
	store := &fakeLearningPathsStore{
		modulesByPath: map[uuid.UUID][]models.Module{},
		userProjectID: "proj-1",
	}
	base := time.Now()
	for i, modules := range pathModules {
		//nolint:exhaustruct //fixture path
		lp := &models.LearningPath{
			ID: uuid.New(), UserID: "user-1", Title: "Path",
			CreatedAt: base.Add(time.Duration(i) * time.Minute),
		}
		store.paths = append(store.paths, lp)
		store.modulesByPath[lp.ID] = modules
	}
	return store
}

func module(items ...models.Item) models.Module {
	//nolint:exhaustruct //fixture module
	return models.Module{ID: uuid.New(), Items: items}
}

func TestSyncUser_OnlyNextItemGetsTask(t *testing.T) {
	store := multiPathStore([]models.Module{
		module(typedItem("read", "read ch1"), typedItem("do", "practice")),
		module(typedItem("do", "build it")),
	})
	svc, mock := newTestTodoistService(store)
	mock.SectionID = "sec-1"

	require.NoError(t, svc.SyncUser(t.Context(), "user-1"))

	assert.Equal(t, []string{"read ch1"}, mock.Contents)
	assert.Equal(t, "proj-1", mock.LastProjectID)
	assert.Equal(t, "sec-1", mock.LastSectionID)
	assert.Equal(t, []string{"Path"}, mock.CreatedSections)
}

func TestSyncUser_OneTaskPerTypeAcrossPaths(t *testing.T) {
	store := multiPathStore(
		[]models.Module{module(typedItem("read", "book A"))},
		[]models.Module{module(typedItem("Read ", "book B"))},
		[]models.Module{module(typedItem("do", "exercise C"))},
	)
	svc, mock := newTestTodoistService(store)

	require.NoError(t, svc.SyncUser(t.Context(), "user-1"))

	assert.Equal(t, []string{"book A", "exercise C"}, mock.Contents)
}

// The path already reading a book keeps the slot over an older path.
func TestSyncUser_HolderKeepsTypeSlot(t *testing.T) {
	store := multiPathStore(
		[]models.Module{module(typedItem("read", "book A"))},
		[]models.Module{module(withTask(typedItem("read", "book B"), "task-b"))},
	)
	svc, mock := newTestTodoistService(store)

	require.NoError(t, svc.SyncUser(t.Context(), "user-1"))

	assert.Empty(t, mock.Contents)
	assert.Empty(t, mock.LastDeletedID)
}

func TestSyncUser_FinishingFreesSlotForWaitingPath(t *testing.T) {
	store := multiPathStore(
		[]models.Module{module(done(withTask(typedItem("read", "book A"), "task-a")))},
		[]models.Module{module(typedItem("read", "book B"))},
	)
	svc, mock := newTestTodoistService(store)

	require.NoError(t, svc.SyncUser(t.Context(), "user-1"))

	assert.Equal(t, "task-a", mock.LastDeletedID)
	assert.Equal(t, []string{"book B"}, mock.Contents)
}

func TestSyncUser_LaterItemTasksRemoved(t *testing.T) {
	store := multiPathStore([]models.Module{
		module(
			withTask(typedItem("read", "ch1"), "t1"),
			withTask(typedItem("do", "ch2"), "t2"),
		),
		module(withTask(typedItem("do", "m2"), "t3")),
	})
	svc, mock := newTestTodoistService(store)

	require.NoError(t, svc.SyncUser(t.Context(), "user-1"))

	assert.Equal(t, "t3", mock.LastDeletedID)
	assert.Equal(t, []string{"", ""}, store.taskIDWrites)
	assert.Empty(t, mock.Contents)
}

func TestSyncUser_PausedPathYieldsSlot(t *testing.T) {
	store := multiPathStore(
		[]models.Module{module(withTask(typedItem("read", "book A"), "task-a"))},
		[]models.Module{module(typedItem("read", "book B"))},
	)
	store.paths[0].Paused = true
	svc, mock := newTestTodoistService(store)

	require.NoError(t, svc.SyncUser(t.Context(), "user-1"))

	assert.Equal(t, "task-a", mock.LastDeletedID)
	assert.Equal(t, []string{"book B"}, mock.Contents)
}

func TestSyncUser_DueFromTypeScheduleUnlessItemOverrides(t *testing.T) {
	read := typedItem("Read", "read the book")
	checkpoint := scheduledItem("pass the quiz", "every sat at 10:00")
	checkpoint.Type = "checkpoint"
	store := multiPathStore(
		[]models.Module{module(read)},
		[]models.Module{module(checkpoint)},
	)
	store.paths[0].ReminderSchedules = map[string]string{"read": "every day at 20:00"}
	svc, mock := newTestTodoistService(store)

	require.NoError(t, svc.SyncUser(t.Context(), "user-1"))

	assert.Equal(t,
		[]string{"every day at 20:00", "every sat at 10:00"}, mock.DueStrings,
	)
}

func TestSyncUser_FirstSyncCreatesSharedProject(t *testing.T) {
	store := multiPathStore([]models.Module{module(nextItem("read"))})
	store.userProjectID = ""
	svc, mock := newTestTodoistService(store)
	mock.ProjectID = "proj-new"
	mock.SectionID = "sec-new"

	require.NoError(t, svc.SyncUser(t.Context(), "user-1"))

	assert.Equal(t, 1, mock.CreatedProjects)
	assert.Equal(t, []string{"proj-new"}, store.userProjectWrites)
	assert.Equal(t, []string{"sec-new"}, store.sectionIDWrites)
	assert.Equal(t, "proj-new", mock.LastProjectID)
}

func TestSyncUser_ExistingSectionReused(t *testing.T) {
	store := multiPathStore([]models.Module{module(nextItem("read"))})
	store.paths[0].TodoistSectionID = new("sec-1")
	svc, mock := newTestTodoistService(store)

	require.NoError(t, svc.SyncUser(t.Context(), "user-1"))

	assert.Zero(t, mock.CreatedProjects)
	assert.Empty(t, mock.CreatedSections)
	assert.Equal(t, "sec-1", mock.LastSectionID)
}

func TestSyncUser_MissingSectionRecreated(t *testing.T) {
	store := multiPathStore([]models.Module{module(nextItem("read"))})
	store.paths[0].TodoistSectionID = new("sec-gone")
	svc, mock := newTestTodoistService(store)
	mock.SectionMissing = true
	mock.SectionID = "sec-2"

	require.NoError(t, svc.SyncUser(t.Context(), "user-1"))

	assert.Equal(t, []string{"sec-2"}, store.sectionIDWrites)
	assert.Equal(t, "sec-2", mock.LastSectionID)
}

// A project deleted in Todoist took its tasks along, so they are recreated.
func TestSyncUser_RecreatedProjectReplacesTasks(t *testing.T) {
	store := multiPathStore([]models.Module{module(withTask(nextItem("read"), "old"))})
	store.paths[0].TodoistSectionID = new("sec-old")
	svc, mock := newTestTodoistService(store)
	mock.ProjectMissing = true
	mock.ProjectID = "proj-new"
	mock.SectionID = "sec-new"

	require.NoError(t, svc.SyncUser(t.Context(), "user-1"))

	assert.Equal(t, []string{"", "task-123"}, store.taskIDWrites)
	assert.Equal(t, "sec-new", mock.LastSectionID)
}

// A pre-shared-project path has its own project deleted and its tasks moved.
func TestSyncUser_LegacyProjectDeletedAndTasksMoved(t *testing.T) {
	store := multiPathStore([]models.Module{module(withTask(nextItem("read"), "old"))})
	store.paths[0].TodoistProjectID = new("legacy")
	svc, mock := newTestTodoistService(store)

	require.NoError(t, svc.SyncUser(t.Context(), "user-1"))

	assert.Equal(t, "legacy", mock.LastDeletedProjectID)
	assert.Equal(t, []string{""}, store.projectIDWrites)
	assert.Empty(t, mock.LastDeletedID)
	assert.Equal(t, []string{"read"}, mock.Contents)
	assert.Equal(t, "proj-1", mock.LastProjectID)
}

func TestSyncUser_NothingOpenCreatesNoProject(t *testing.T) {
	store := multiPathStore([]models.Module{module(done(nextItem("read")))})
	store.userProjectID = ""
	svc, mock := newTestTodoistService(store)

	require.NoError(t, svc.SyncUser(t.Context(), "user-1"))
	assert.Zero(t, mock.CreatedProjects)
}

func TestSyncUser_NotConnectedIsNoOp(t *testing.T) {
	store := multiPathStore([]models.Module{module(nextItem("read"))})
	svc, mock := newTestTodoistService(store)
	//nolint:exhaustruct //fixture connections
	svc.oauthRepo = &fakeConnections{connected: false}

	require.NoError(t, svc.SyncUser(t.Context(), "user-1"))
	assert.Empty(t, mock.Contents)
}

func TestSyncUser_PropagatesErrors(t *testing.T) {
	dbErr := errors.New("db down")
	t.Run("list", func(t *testing.T) {
		store := multiPathStore()
		store.listErr = dbErr
		svc, _ := newTestTodoistService(store)
		assert.ErrorIs(t, svc.SyncUser(t.Context(), "user-1"), dbErr)
	})
	t.Run("modules", func(t *testing.T) {
		store := multiPathStore([]models.Module{module(nextItem("read"))})
		store.getModulesErr = dbErr
		svc, _ := newTestTodoistService(store)
		assert.ErrorIs(t, svc.SyncUser(t.Context(), "user-1"), dbErr)
	})
	// Each case reaches a different Todoist call, which fails.
	for name, mutate := range map[string]func(lp *models.LearningPath, it *models.Item){
		"create task": func(*models.LearningPath, *models.Item) {},
		"delete task": func(_ *models.LearningPath, it *models.Item) {
			it.Completed = true
			it.TodoistTaskID = new("t")
		},
		"legacy project": func(lp *models.LearningPath, _ *models.Item) {
			lp.TodoistProjectID = new("legacy")
		},
		"section exists": func(lp *models.LearningPath, _ *models.Item) {
			lp.TodoistSectionID = new("sec")
		},
	} {
		t.Run(name, func(t *testing.T) {
			store := multiPathStore([]models.Module{module(nextItem("read"))})
			lp := store.paths[0]
			mutate(lp, &store.modulesByPath[lp.ID][0].Items[0])
			svc, mock := newTestTodoistService(store)
			mock.Err = errors.New("todoist down")
			assert.ErrorIs(t, svc.SyncUser(t.Context(), "user-1"), mock.Err)
		})
	}
	t.Run("create project", func(t *testing.T) {
		store := multiPathStore([]models.Module{module(nextItem("read"))})
		store.userProjectID = ""
		svc, mock := newTestTodoistService(store)
		mock.Err = errors.New("todoist down")
		assert.ErrorIs(t, svc.SyncUser(t.Context(), "user-1"), mock.Err)
		assert.Empty(t, store.userProjectWrites)
	})
}

func TestResyncAll_SyncsEveryUserAndJoinsErrors(t *testing.T) {
	store := multiPathStore([]models.Module{module(nextItem("read"))})
	svc, mock := newTestTodoistService(store)
	//nolint:exhaustruct //fixture connections
	svc.oauthRepo = &fakeConnections{connected: true, userIDs: []string{"a", "b"}}

	require.NoError(t, svc.ResyncAll(t.Context()))
	// The fake store serves the same path to both users.
	assert.Equal(t, []string{"read", "read"}, mock.Contents)

	mock.Err = errors.New("todoist down")
	err := svc.ResyncAll(t.Context())
	assert.ErrorIs(t, err, mock.Err)
	assert.Contains(t, err.Error(), "user a")
	assert.Contains(t, err.Error(), "user b")

	listErr := errors.New("db down")
	//nolint:exhaustruct //fixture connections
	svc.oauthRepo = &fakeConnections{connected: true, listErr: listErr}
	assert.ErrorIs(t, svc.ResyncAll(t.Context()), listErr)
}

func TestDeletePath_RemovesSection(t *testing.T) {
	store := multiPathStore([]models.Module{module(withTask(nextItem("read"), "t-1"))})
	lp := store.paths[0]
	lp.TodoistSectionID = new("sec-1")
	svc, mock := newTestTodoistService(store)

	modules := store.modulesByPath[lp.ID]
	require.NoError(t, svc.DeletePath(t.Context(), "user-1", lp, modules))

	assert.Equal(t, "t-1", mock.LastDeletedID)
	assert.Equal(t, "sec-1", mock.LastDeletedSectionID)
	assert.Empty(t, mock.LastDeletedProjectID)
}

func readingBook(description string, progress int) models.Item {
	it := typedItem("read", description)
	it.LinkedBookID = new(uuid.New())
	//nolint:exhaustruct //only progress matters
	it.LinkedBook = &models.LinkedBook{ProgressPercent: progress}
	return it
}

func TestSyncUser_PrefersStartedBookInActiveModule(t *testing.T) {
	store := multiPathStore([]models.Module{
		module(
			readingBook("unstarted", 0),
			readingBook("a bit", 17),
			readingBook("further", 36),
			typedItem("do", "apply"),
		),
		module(readingBook("next module", 80)),
	})
	svc, mock := newTestTodoistService(store)

	require.NoError(t, svc.SyncUser(t.Context(), "user-1"))

	assert.Equal(t, []string{"further"}, mock.Contents)
}

func TestSyncUser_NoStartedBookTakesFirstIncomplete(t *testing.T) {
	store := multiPathStore([]models.Module{
		module(done(nextItem("done")), readingBook("unstarted", 0), typedItem("do", "x")),
	})
	svc, mock := newTestTodoistService(store)

	require.NoError(t, svc.SyncUser(t.Context(), "user-1"))

	assert.Equal(t, []string{"unstarted"}, mock.Contents)
}
