package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/oauth2"

	"tools.xdoubleu.com/apps/learningpaths/internal/repositories"
	"tools.xdoubleu.com/internal/database"
	sharedmodels "tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/oauthconn"
	"tools.xdoubleu.com/internal/todoist"
)

// ErrTodoistNotConnected is returned when the caller hasn't connected Todoist;
// callers map it to FailedPrecondition, not a 500.
var ErrTodoistNotConnected = oauthconn.ErrNotConnected

// TodoistService connects a user's own Todoist account and sends single
// learning-path items to it as tasks, one-way. Strictly per-user; no admin
// bypass.
type TodoistService struct {
	oauthRepo     *repositories.OAuthConnectionsRepository
	learningPaths learningPathsStore
	conf          *oauth2.Config
	state         *oauthconn.StateStore
	// newClient is a field so tests can substitute a mock Client.
	newClient func(oauthconn.TokenFunc) todoist.Client
}

func NewTodoistService(
	oauthRepo *repositories.OAuthConnectionsRepository,
	learningPaths learningPathsStore,
	conf *oauth2.Config,
	state *oauthconn.StateStore,
) *TodoistService {
	return &TodoistService{
		oauthRepo:     oauthRepo,
		learningPaths: learningPaths,
		conf:          conf,
		state:         state,
		newClient:     todoist.NewClient,
	}
}

// AuthorizeURL issues a CSRF state for userID and returns the authorize URL.
// The callback leg is plain HTTP (routes.go).
func (s *TodoistService) AuthorizeURL(userID string) string {
	state := s.state.New(sharedmodels.OAuthProviderTodoist, userID)
	return s.conf.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

// HandleCallback consumes state, exchanges code, and stores the connection,
// returning the userID the state was issued for.
func (s *TodoistService) HandleCallback(
	ctx context.Context, state, code string,
) (string, error) {
	provider, userID, ok := s.state.Consume(state)
	if !ok || provider != sharedmodels.OAuthProviderTodoist {
		return "", errors.New("todoist: invalid or expired oauth state")
	}

	tok, err := s.conf.Exchange(ctx, code)
	if err != nil {
		return "", fmt.Errorf("todoist: oauth exchange failed: %w", err)
	}

	if err = s.oauthRepo.Upsert(
		ctx, userID, sharedmodels.OAuthProviderTodoist, tok,
	); err != nil {
		return "", err
	}

	return userID, nil
}

// Disconnect removes userID's stored Todoist connection, if any.
func (s *TodoistService) Disconnect(ctx context.Context, userID string) error {
	return s.oauthRepo.Delete(ctx, userID, sharedmodels.OAuthProviderTodoist)
}

// Status reports whether userID is connected and since when; "not connected"
// is not an error.
func (s *TodoistService) Status(
	ctx context.Context, userID string,
) (bool, time.Time, error) {
	conn, err := s.oauthRepo.GetStatus(
		ctx, userID, sharedmodels.OAuthProviderTodoist,
	)
	if errors.Is(err, database.ErrResourceNotFound) {
		return false, time.Time{}, nil
	}
	if err != nil {
		return false, time.Time{}, err
	}
	return true, conn.ConnectedAt, nil
}

// SendItem creates a Todoist task from itemID's description, prefixed with
// its path's title; 404 on foreign ownership. Returns ErrTodoistNotConnected
// if userID hasn't connected Todoist.
func (s *TodoistService) SendItem(
	ctx context.Context, userID string, itemID uuid.UUID,
) (string, error) {
	item, err := s.learningPaths.GetItemForUser(ctx, itemID, userID)
	if err != nil {
		return "", err
	}

	tokenFn := oauthconn.NewTokenFunc(
		s.oauthRepo.ForUser(userID), sharedmodels.OAuthProviderTodoist, s.conf,
	)
	client := s.newClient(tokenFn)

	content := fmt.Sprintf("%s: %s", item.PathTitle, item.Item.Description)
	return client.CreateTask(ctx, content, "")
}

// SetOAuthConfigForTest overrides the Todoist OAuth2 config, e.g. to point
// TokenURL at an httptest server. Tests only.
func (s *TodoistService) SetOAuthConfigForTest(conf *oauth2.Config) {
	s.conf = conf
}
