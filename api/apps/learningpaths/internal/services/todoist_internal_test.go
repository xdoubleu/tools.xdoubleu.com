package services

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"golang.org/x/oauth2"

	"tools.xdoubleu.com/apps/learningpaths/internal/mocks"
	"tools.xdoubleu.com/apps/learningpaths/internal/models"
	"tools.xdoubleu.com/apps/learningpaths/internal/repositories"
	"tools.xdoubleu.com/apps/learningpaths/internal/todoist"
	"tools.xdoubleu.com/internal/kit/database"
	sharedmodels "tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/platform/oauthconn"
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

// fakeConnections is an in-memory TodoistConnections; the mock Client never
// invokes the token function, so ForUser returns an unused store.
type fakeConnections struct {
	connected bool
	statusErr error
	// requestedScope is the connection's recorded scope; empty means unknown,
	// which oauthconn treats as covering every scope.
	requestedScope string
	// userIDs is TodoistUserIDs, failing with listErr.
	userIDs []string
	listErr error
}

func (f *fakeConnections) TodoistUserIDs(_ context.Context) ([]string, error) {
	return f.userIDs, f.listErr
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
