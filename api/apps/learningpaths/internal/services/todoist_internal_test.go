package services

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"tools.xdoubleu.com/apps/learningpaths/internal/mocks"
	"tools.xdoubleu.com/apps/learningpaths/internal/models"
	"tools.xdoubleu.com/apps/learningpaths/internal/repositories"
	sharedmodels "tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/oauthconn"
	"tools.xdoubleu.com/internal/todoist"
)

func newTestTodoistService(
	store *fakeLearningPathsStore,
) (*TodoistService, *mocks.MockTodoistClient) {
	//nolint:exhaustruct //endpoint URLs, not credentials
	conf := &oauth2.Config{ClientID: "id", ClientSecret: "secret"}
	// A repository with a nil db/sealer is safe here: SendItem's own code
	// path never calls either (ForUser only builds a value, it doesn't
	// query; the mock Client swapped in below never invokes the TokenFunc
	// that would).
	repo := repositories.NewOAuthConnectionsRepository(nil, nil)
	svc := NewTodoistService(repo, store, conf, oauthconn.NewStateStore())

	mock := mocks.NewMockTodoistClient("task-123")
	svc.newClient = func(oauthconn.TokenFunc) todoist.Client { return mock }
	return svc, mock
}

func TestSendItem_BuildsContentFromPathTitleAndDescription(t *testing.T) {
	//nolint:exhaustruct //only fields relevant to this test
	store := &fakeLearningPathsStore{
		item: &models.ItemForTask{
			Item:      models.Item{Description: "Read chapter 3"},
			PathTitle: "Learn Go",
		},
	}
	svc, mock := newTestTodoistService(store)

	taskID, err := svc.SendItem(t.Context(), "user-1", uuid.New())
	require.NoError(t, err)
	assert.Equal(t, "task-123", taskID)
	assert.Equal(t, "Learn Go: Read chapter 3", mock.LastContent)
	assert.Empty(t, mock.LastDueString)
}

func TestSendItem_PropagatesItemLookupError(t *testing.T) {
	lookupErr := errors.New("item lookup failed")
	//nolint:exhaustruct //only fields relevant to this test
	store := &fakeLearningPathsStore{getItemErr: lookupErr}
	svc, _ := newTestTodoistService(store)

	_, err := svc.SendItem(t.Context(), "user-1", uuid.New())
	assert.ErrorIs(t, err, lookupErr)
}

func TestSendItem_PropagatesTodoistClientError(t *testing.T) {
	clientErr := errors.New("todoist API error")
	//nolint:exhaustruct //only fields relevant to this test
	store := &fakeLearningPathsStore{
		item: &models.ItemForTask{
			Item:      models.Item{Description: "Read chapter 3"},
			PathTitle: "Learn Go",
		},
	}
	svc, mock := newTestTodoistService(store)
	mock.Err = clientErr

	_, err := svc.SendItem(t.Context(), "user-1", uuid.New())
	assert.ErrorIs(t, err, clientErr)
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

	_, err := svc.HandleCallback(t.Context(), "not-a-real-state", "code")
	assert.Error(t, err)
}

// TestHandleCallback_ExchangeFailure covers the token-exchange error branch
// without a real Todoist network call: TokenURL points at a closed local
// port, so the connection is refused immediately.
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
	_, err := svc.HandleCallback(t.Context(), validState, "some-code")
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
