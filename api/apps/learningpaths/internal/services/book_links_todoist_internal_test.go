package services

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books"
	"tools.xdoubleu.com/apps/learningpaths/internal/mocks"
	"tools.xdoubleu.com/apps/learningpaths/internal/models"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
	iapp "tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/logging"
)

func externalBookPath() models.LearningPath {
	//nolint:exhaustruct //fixture sets only the fields under test
	return models.LearningPath{
		ID:    uuid.New(),
		Title: "Architecture",
		Modules: []models.Module{{
			Title: "Week 1",
			Items: []models.Item{{
				Type:        "read",
				Description: "Team Topologies",
				ExternalBook: &models.ExternalBookRef{
					Provider: "hardcover", ProviderID: "9781942788812",
				},
			}},
		}},
	}
}

// newConnectedService wires a LearningPathService to a connected Todoist
// mock, with SyncPath deriving book completion like services.New does.
func newConnectedService(
	store *fakeLearningPathsStore, bookLookup *fakeBookLookup,
) (*LearningPathService, *mocks.MockTodoistClient) {
	todoistSvc, mock := newTestTodoistService(store)
	//nolint:exhaustruct //feeds unused on these paths
	svc := &LearningPathService{
		logger: logging.NewNopLogger(), repo: store, books: bookLookup, todoist: todoistSvc,
	}
	todoistSvc.resolveItems = svc.resolveItemLinks
	return svc, mock
}

func TestCreate_ExternalBookAddedToLearnShelfAndLinked(t *testing.T) {
	bookID := uuid.New()
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{lp: newFixture()}
	//nolint:exhaustruct //unset fields are the fixture defaults
	bookLookup := &fakeBookLookup{ensureID: bookID, book: &booksv1.UserBook{}}
	svc := newTestService(store)
	svc.books = bookLookup

	created, err := svc.Create(t.Context(), "owner", externalBookPath())
	require.NoError(t, err)

	assert.Equal(t, []string{models.LearnShelf}, bookLookup.ensureShelves)
	require.NotNil(t, created.Modules[0].Items[0].LinkedBookID)
	assert.Equal(t, bookID, *created.Modules[0].Items[0].LinkedBookID)
}

func TestCreate_LinkedBookIDWinsOverExternalBook(t *testing.T) {
	ownedID := uuid.New()
	lp := externalBookPath()
	lp.Modules[0].Items[0].LinkedBookID = &ownedID
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{lp: newFixture()}
	//nolint:exhaustruct //unset fields are the fixture defaults
	bookLookup := &fakeBookLookup{book: &booksv1.UserBook{}}
	svc := newTestService(store)
	svc.books = bookLookup

	created, err := svc.Create(t.Context(), "owner", lp)
	require.NoError(t, err)

	assert.Empty(t, bookLookup.ensureShelves)
	assert.Equal(t, ownedID, *created.Modules[0].Items[0].LinkedBookID)
}

func TestCreate_UnresolvableExternalBookIs400(t *testing.T) {
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{lp: newFixture()}
	svc := newTestService(store)
	//nolint:exhaustruct //unset fields are the fixture defaults
	svc.books = &fakeBookLookup{ensureErr: books.ErrExternalBookNotFound}

	_, err := svc.Create(t.Context(), "owner", externalBookPath())

	var httpErr *iapp.HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusBadRequest, httpErr.Status)
	assert.Contains(t, httpErr.Message, "hardcover/9781942788812")
	assert.False(t, store.modulesReplaced)
}

func TestUpdate_ExternalBookInfraErrorPropagates(t *testing.T) {
	infraErr := errors.New("provider down")
	lp := newFixture()
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{lp: lp}
	svc := newTestService(store)
	//nolint:exhaustruct //unset fields are the fixture defaults
	svc.books = &fakeBookLookup{ensureErr: infraErr}
	update := externalBookPath()
	update.ID = lp.ID

	err := svc.Update(t.Context(), lp.UserID, update)
	require.ErrorIs(t, err, infraErr)
	assert.False(t, store.updated)
}

// TestUpdate_ClearsOldTasksThenActivatesActiveModule: replaced items would
// orphan their tasks, so Update deletes them and the sync recreates the
// active module's.
func TestUpdate_ClearsOldTasksThenActivatesActiveModule(t *testing.T) {
	lp := newFixture()
	lp.Title = "Architecture"
	old := nextItem("old step")
	oldTask := "old-1"
	old.TodoistTaskID = &oldTask
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{
		lp:      lp,
		modules: []models.Module{{ID: uuid.New(), Items: []models.Item{old}}},
	}
	//nolint:exhaustruct //unset fields are the fixture defaults
	svc, mock := newConnectedService(store, &fakeBookLookup{})

	//nolint:exhaustruct //fixture sets only the fields under test
	update := models.LearningPath{
		ID:      lp.ID,
		Title:   lp.Title,
		Modules: []models.Module{{Items: []models.Item{nextItem("new step")}}},
	}
	require.NoError(t, svc.Update(t.Context(), lp.UserID, update))

	assert.Equal(t, "old-1", mock.LastDeletedID)
	assert.Equal(t, "new step", mock.LastContent)
	assert.Equal(t, []string{"", "task-123"}, store.taskIDWrites)
}

func TestUpdate_ModuleLoadErrorSkipsTodoistButSucceeds(t *testing.T) {
	lp := newFixture()
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{lp: lp, getModulesErr: errors.New("db")}
	//nolint:exhaustruct //unset fields are the fixture defaults
	svc, mock := newConnectedService(store, &fakeBookLookup{})
	var logs bytes.Buffer
	svc.logger = slog.New(slog.NewTextHandler(&logs, nil))

	require.NoError(t, svc.Update(t.Context(), lp.UserID, *lp))
	assert.True(t, store.modulesReplaced)
	assert.Empty(t, mock.LastDeletedID)
	assert.Contains(t, logs.String(), "level=ERROR")
	assert.Contains(t, logs.String(), "todoist sync failed")
}

// TestSyncPath_FinishedBookCompletesModule: a book-linked item's stored flag
// stays false, so the pipeline must derive completion from the book.
func TestSyncPath_FinishedBookCompletesModule(t *testing.T) {
	lp := newFixture()
	lp.Title = "Architecture"
	bookID := uuid.New()
	bookTask := "book-task"
	book := nextItem("read the book")
	book.LinkedBookID = &bookID
	book.TodoistTaskID = &bookTask
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{
		lp: lp,
		modules: []models.Module{
			{ID: uuid.New(), Items: []models.Item{book}},
			{ID: uuid.New(), SortOrder: 1, Items: []models.Item{nextItem("apply it")}},
		},
	}
	//nolint:exhaustruct //unset fields are the fixture defaults
	svc, mock := newConnectedService(store, &fakeBookLookup{
		book: &booksv1.UserBook{ProgressPercent: 100},
	})

	require.NoError(t, svc.todoist.SyncPath(t.Context(), lp.UserID, lp.ID))
	assert.Equal(t, "book-task", mock.LastDeletedID)
	assert.Equal(t, "apply it", mock.LastContent)
}

func TestSyncPath_PropagatesResolveError(t *testing.T) {
	lookupErr := errors.New("books down")
	lp := newFixture()
	bookID := uuid.New()
	item := nextItem("read")
	item.LinkedBookID = &bookID
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{
		lp:      lp,
		modules: []models.Module{{ID: uuid.New(), Items: []models.Item{item}}},
	}
	//nolint:exhaustruct //unset fields are the fixture defaults
	svc, _ := newConnectedService(store, &fakeBookLookup{
		genericErr: lookupErr, errAfterCall: 1,
	})

	err := svc.todoist.SyncPath(t.Context(), lp.UserID, lp.ID)
	assert.ErrorIs(t, err, lookupErr)
}

func TestClearTasks_NotConnectedIsNoOp(t *testing.T) {
	task := "t-1"
	item := nextItem("read")
	item.TodoistTaskID = &task
	//nolint:exhaustruct //unset fields are the fixture defaults
	store := &fakeLearningPathsStore{}
	svc, mock := newTestTodoistService(store)
	//nolint:exhaustruct //fixture connections
	svc.oauthRepo = &fakeConnections{connected: false}

	//nolint:exhaustruct //only Items matter
	modules := []models.Module{{Items: []models.Item{item}}}
	err := svc.ClearTasks(t.Context(), "user-1", modules)
	require.NoError(t, err)
	assert.Empty(t, mock.LastDeletedID)
}
