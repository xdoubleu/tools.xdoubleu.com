package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"tools.xdoubleu.com/apps/learningpaths/internal/mocks"
	"tools.xdoubleu.com/apps/learningpaths/internal/models"
	"tools.xdoubleu.com/apps/learningpaths/internal/repositories"
	"tools.xdoubleu.com/internal/database"
	sharedmodels "tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/oauthconn"
	"tools.xdoubleu.com/internal/todoist"
)

func newTestTodoistService(
	store *fakeLearningPathsStore,
) (*TodoistService, *mocks.MockTodoistClient) {
	//nolint:exhaustruct //endpoint URLs, not credentials
	conf := &oauth2.Config{ClientID: "id", ClientSecret: "secret"}
	conns := &fakeConnections{connected: true} //nolint:exhaustruct // statusErr unused
	svc := NewTodoistService(conns, store, conf, oauthconn.NewStateStore())

	mock := mocks.NewMockTodoistClient("task-123")
	svc.newClient = func(oauthconn.TokenFunc) todoist.Client { return mock }
	return svc, mock
}

// fakeConnections is an in-memory todoistConnections; the mock Client never
// invokes the token function, so ForUser returns an unused store.
type fakeConnections struct {
	connected bool
	statusErr error
	// requestedScope is the connection's recorded scope; empty means unknown,
	// which oauthconn treats as covering every scope.
	requestedScope string
}

func (f *fakeConnections) Status(
	_ context.Context, _ string,
) (*sharedmodels.OAuthConnection, error) {
	if f.statusErr != nil {
		return nil, f.statusErr
	}
	if !f.connected {
		return nil, database.ErrResourceNotFound
	}
	//nolint:exhaustruct //only ConnectedAt/RequestedScope are read
	return &sharedmodels.OAuthConnection{
		ConnectedAt: time.Now(), RequestedScope: f.requestedScope,
	}, nil
}

func (f *fakeConnections) Upsert(
	_ context.Context, _ string, _ sharedmodels.OAuthProvider, _ *oauth2.Token,
	requestedScope string,
) error {
	f.requestedScope = requestedScope
	return nil
}

func (f *fakeConnections) Delete(
	_ context.Context, _ string, _ sharedmodels.OAuthProvider,
) error {
	return nil
}

func (f *fakeConnections) ForUser(_ string) repositories.UserScopedOAuthStore {
	return repositories.UserScopedOAuthStore{}
}

func TestHandleCallback_UnknownStateErrors(t *testing.T) {
	//nolint:exhaustruct //endpoint URLs, not credentials
	conf := &oauth2.Config{ClientID: "id", ClientSecret: "secret"}
	repo := repositories.NewOAuthConnectionsRepository(nil, nil)
	//nolint:exhaustruct //only fields relevant to this test
	svc := NewTodoistService(
		repo,
		&fakeLearningPathsStore{},
		conf,
		oauthconn.NewStateStore(),
	)

	_, err := svc.HandleCallback(t.Context(), "user-1", "not-a-real-state", "code")
	assert.Error(t, err)
}

// TestHandleCallback_ExchangeFailure: TokenURL points at a closed port.
func TestHandleCallback_ExchangeFailure(t *testing.T) {
	//nolint:exhaustruct //endpoint URLs, not credentials
	conf := &oauth2.Config{
		ClientID:     "id",
		ClientSecret: "secret",
		Endpoint:     oauth2.Endpoint{TokenURL: "http://127.0.0.1:1/token"},
	}
	repo := repositories.NewOAuthConnectionsRepository(nil, nil)
	state := oauthconn.NewStateStore()
	//nolint:exhaustruct //only fields relevant to this test
	svc := NewTodoistService(repo, &fakeLearningPathsStore{}, conf, state)

	validState := state.New(sharedmodels.OAuthProviderTodoist, "user-1")
	_, err := svc.HandleCallback(t.Context(), "user-1", validState, "some-code")
	assert.Error(t, err)
}

func TestAuthorizeURL_IncludesState(t *testing.T) {
	//nolint:exhaustruct //endpoint URLs, not credentials
	conf := &oauth2.Config{
		ClientID: "id",
		Endpoint: oauth2.Endpoint{AuthURL: "https://app.todoist.com/oauth/authorize"},
	}
	repo := repositories.NewOAuthConnectionsRepository(nil, nil)
	//nolint:exhaustruct //only fields relevant to this test
	svc := NewTodoistService(
		repo,
		&fakeLearningPathsStore{},
		conf,
		oauthconn.NewStateStore(),
	)

	url := svc.AuthorizeURL("user-1")
	assert.Contains(t, url, "https://app.todoist.com/oauth/authorize")
	assert.Contains(t, url, "client_id=id")
	assert.Contains(t, url, "state=")
}

func nextItem(description string) models.Item {
	//nolint:exhaustruct //fixture sets only the fields under test
	return models.Item{ID: uuid.New(), Description: description}
}

// TestSyncPath_ConnectedActivatesFirstModule covers the "path created"
// surface: the first module's items each get a task.
func TestSyncPath_ConnectedActivatesFirstModule(t *testing.T) {
	//nolint:exhaustruct //fixture path
	p := &models.LearningPath{ID: uuid.New(), UserID: "user-1", Title: "Learn Go"}
	//nolint:exhaustruct //fixture store
	store := &fakeLearningPathsStore{
		lp: p,
		modules: []models.Module{
			models.Module{
				ID: uuid.New(), LearningPathID: p.ID, SortOrder: 0,
				Items: []models.Item{nextItem("read ch1"), nextItem("practice")},
			},
		},
	}
	svc, mock := newTestTodoistService(store)

	require.NoError(t, svc.SyncPath(t.Context(), "user-1", p.ID))
	assert.Equal(t, 2, len(store.taskIDWrites))
	assert.Equal(t, "practice", mock.LastContent)
}

func TestSyncPath_NotConnected_IsNoOp(t *testing.T) {
	//nolint:exhaustruct //fixture store
	store := &fakeLearningPathsStore{
		lp: &models.LearningPath{
			ID:     uuid.New(),
			UserID: "user-1",
			Title:  "Learn Go",
		}, //nolint:exhaustruct // fixture omits unused fields
		modules: []models.Module{
			models.Module{
				ID:        uuid.New(),
				SortOrder: 0,
				Items:     []models.Item{nextItem("read")},
			}, //nolint:exhaustruct // fixture omits unused fields
		},
	}
	svc, mock := newTestTodoistService(store)
	//nolint:exhaustruct //fixture connections
	svc.oauthRepo = &fakeConnections{connected: false}

	require.NoError(t, svc.SyncPath(t.Context(), "user-1", uuid.New()))
	assert.Empty(t, store.taskIDWrites)
	assert.Empty(t, mock.LastContent)
}

// TestSyncPath_ModuleCompleteActivatesNext covers "completing module N": its
// tasks are deleted and the next module's items get tasks.
func TestSyncPath_ModuleCompleteActivatesNext(t *testing.T) {
	//nolint:exhaustruct //fixture path
	p := &models.LearningPath{ID: uuid.New(), UserID: "user-1", Title: "Learn Go"}
	complete := nextItem("read ch1")
	complete.Completed = true
	completeID := "task-1"
	complete.TodoistTaskID = &completeID
	//nolint:exhaustruct //fixture store
	store := &fakeLearningPathsStore{
		lp: p,
		modules: []models.Module{
			models.Module{
				ID: uuid.New(), SortOrder: 0,
				Items: []models.Item{complete},
			},
			models.Module{
				ID: uuid.New(), SortOrder: 1,
				Items: []models.Item{nextItem("read ch2")},
			},
		},
	}
	svc, mock := newTestTodoistService(store)

	require.NoError(t, svc.SyncPath(t.Context(), "user-1", p.ID))
	assert.Equal(t, "task-1", mock.LastDeletedID)
	// one delete (clear the completed module's task) + one create (module 2).
	assert.Equal(t, 2, len(store.taskIDWrites))
	assert.Equal(t, "read ch2", mock.LastContent)
}

// TestSyncPath_IdempotentKeepsExistingTask: an item already carrying a task
// id in the active module is not re-created.
func TestSyncPath_IdempotentKeepsExistingTask(t *testing.T) {
	projectID := "proj-1"
	//nolint:exhaustruct //fixture path
	p := &models.LearningPath{
		ID: uuid.New(), UserID: "user-1", Title: "Learn Go",
		TodoistProjectID: &projectID,
	}
	item := nextItem("read ch1")
	existing := "task-9"
	item.TodoistTaskID = &existing
	//nolint:exhaustruct //fixture store
	store := &fakeLearningPathsStore{
		lp: p,
		modules: []models.Module{
			models.Module{
				ID:        uuid.New(),
				SortOrder: 0,
				Items:     []models.Item{item},
			}, //nolint:exhaustruct // fixture omits unused fields
		},
	}
	svc, mock := newTestTodoistService(store)

	require.NoError(t, svc.SyncPath(t.Context(), "user-1", p.ID))
	assert.Empty(t, store.taskIDWrites)
	assert.Empty(t, mock.LastContent)
	assert.Zero(t, mock.CreatedProjects)
}

// TestSyncPath_AllModulesDoneDeletesEveryTask: with no active module left, all
// stored tasks are removed and none created.
func TestSyncPath_AllModulesDoneDeletesEveryTask(t *testing.T) {
	//nolint:exhaustruct //fixture path
	p := &models.LearningPath{ID: uuid.New(), UserID: "user-1", Title: "Learn Go"}
	item := nextItem("read ch1")
	item.Completed = true
	done := "task-5"
	item.TodoistTaskID = &done
	//nolint:exhaustruct //fixture store
	store := &fakeLearningPathsStore{
		lp: p,
		modules: []models.Module{
			models.Module{
				ID:        uuid.New(),
				SortOrder: 0,
				Items:     []models.Item{item},
			}, //nolint:exhaustruct // fixture omits unused fields
		},
	}
	svc, mock := newTestTodoistService(store)

	require.NoError(t, svc.SyncPath(t.Context(), "user-1", p.ID))
	assert.Equal(t, "task-5", mock.LastDeletedID)
	assert.Equal(t, []string{""}, store.taskIDWrites)
}

func TestSyncPath_RejectsForeignUser(t *testing.T) {
	//nolint:exhaustruct //fixture path
	p := &models.LearningPath{ID: uuid.New(), UserID: "someone-else", Title: "Learn Go"}
	//nolint:exhaustruct //fixture store
	store := &fakeLearningPathsStore{
		lp: p,
		modules: []models.Module{
			models.Module{
				ID:        uuid.New(),
				SortOrder: 0,
				Items:     []models.Item{nextItem("read")},
			}, //nolint:exhaustruct // fixture omits unused fields
		},
	}
	svc, _ := newTestTodoistService(store)

	err := svc.SyncPath(t.Context(), "user-1", p.ID)
	assert.ErrorIs(t, err, database.ErrResourceNotFound)
}

func TestSyncPath_PropagatesDeleteError(t *testing.T) {
	//nolint:exhaustruct //fixture path
	p := &models.LearningPath{ID: uuid.New(), UserID: "user-1", Title: "Learn Go"}
	item := nextItem("read ch1")
	item.Completed = true
	done := "task-5"
	item.TodoistTaskID = &done
	//nolint:exhaustruct //fixture store
	store := &fakeLearningPathsStore{
		lp: p,
		modules: []models.Module{
			models.Module{
				ID:        uuid.New(),
				SortOrder: 0,
				Items:     []models.Item{item},
			}, //nolint:exhaustruct // fixture omits unused fields
		},
	}
	svc, mock := newTestTodoistService(store)
	mock.Err = errors.New("delete failed")

	assert.Error(t, svc.SyncPath(t.Context(), "user-1", p.ID))
}

func TestSyncPath_PropagatesCreateError(t *testing.T) {
	//nolint:exhaustruct //fixture path
	p := &models.LearningPath{ID: uuid.New(), UserID: "user-1", Title: "Learn Go"}
	//nolint:exhaustruct //fixture store
	store := &fakeLearningPathsStore{
		lp: p,
		modules: []models.Module{
			models.Module{
				ID:        uuid.New(),
				SortOrder: 0,
				Items:     []models.Item{nextItem("read")},
			}, //nolint:exhaustruct // fixture omits unused fields
		},
	}
	svc, mock := newTestTodoistService(store)
	mock.Err = errors.New("create failed")

	assert.Error(t, svc.SyncPath(t.Context(), "user-1", p.ID))
}

func TestSyncPath_PropagatesModuleLoadError(t *testing.T) {
	//nolint:exhaustruct //fixture store seeded with a load error
	store := &fakeLearningPathsStore{
		lp: &models.LearningPath{
			ID:     uuid.New(),
			UserID: "user-1",
			Title:  "Learn Go",
		}, //nolint:exhaustruct // fixture path
		getModulesErr: errors.New("modules load failed"),
	}
	svc, _ := newTestTodoistService(store)

	assert.Error(t, svc.SyncPath(t.Context(), "user-1", uuid.New()))
}

// A state issued to another user can't link the session user's Todoist.
func TestHandleCallback_StateOfOtherUserRejected(t *testing.T) {
	//nolint:exhaustruct //endpoint URLs, not credentials
	conf := &oauth2.Config{ClientID: "id", ClientSecret: "secret"}
	repo := repositories.NewOAuthConnectionsRepository(nil, nil)
	state := oauthconn.NewStateStore()
	//nolint:exhaustruct //only fields relevant to this test
	svc := NewTodoistService(repo, &fakeLearningPathsStore{}, conf, state)

	attackerState := state.New(sharedmodels.OAuthProviderTodoist, "attacker")
	_, err := svc.HandleCallback(t.Context(), "victim", attackerState, "code")
	assert.ErrorContains(t, err, "invalid or expired oauth state")
}
