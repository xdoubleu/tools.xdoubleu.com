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

// ErrTodoistNotConnected is returned by SendItem/Status-adjacent calls when
// the caller hasn't connected their own Todoist account yet — callers map
// this to a client-actionable error (ConnectRPC FailedPrecondition), not a
// 500, since it's an expected state, not a bug.
var ErrTodoistNotConnected = oauthconn.ErrNotConnected

// TodoistService lets a user connect their own Todoist account and send a
// single learning-path item to it as a task (issue #1475), one-way — no
// sync-back, per #1471's "Not this". Every method is scoped by the caller's
// own userID; there is no cross-user access here at all (unlike
// LearningPathService's family-less-but-still-per-app RBAC, this has no
// admin bypass either — an admin's own Todoist connection, if any, is a
// user like any other).
type TodoistService struct {
	oauthRepo     *repositories.OAuthConnectionsRepository
	learningPaths learningPathsStore
	conf          *oauth2.Config
	state         *oauthconn.StateStore
	// newClient builds a todoist.Client from a live TokenFunc — a field
	// (not a direct todoist.NewClient call) so tests can substitute a mock
	// Client without a network round trip.
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

// AuthorizeURL issues a fresh CSRF state for userID and returns the URL the
// client should navigate the browser to. The callback leg that completes
// the flow is plain HTTP, not ConnectRPC — see routes.go.
func (s *TodoistService) AuthorizeURL(userID string) string {
	state := s.state.New(sharedmodels.OAuthProviderTodoist, userID)
	return s.conf.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

// HandleCallback consumes state, exchanges code for a token, and stores the
// connection. Returns the userID the state was issued for, so the plain
// HTTP callback route can log/redirect appropriately.
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

// Status reports whether userID has a Todoist connection and, if so, when it
// was established. Never returns an error for "not connected" — that's a
// normal, expected reply for this RPC.
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

// SendItem creates a Todoist task from itemID's description (prefixed with
// its owning path's title for context), for whichever user owns it — 404 on
// foreign ownership, same rule as every other per-user lookup in this app.
// Returns ErrTodoistNotConnected if userID hasn't connected Todoist.
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

// SetOAuthConfigForTest overrides the OAuth2 config used for the Todoist
// authorize/exchange flow. Test-only: lets a test point TokenURL at a local
// httptest server instead of Todoist's real endpoints, the same idea as
// cmd/api's admin OAuth tests stubbing the equivalent GitHub/Sentry leg via
// withStubProvider — that flow's provider table is a package-level var
// swappable in-place, but this service's conf is a private field, so the
// override needs an explicit seam.
func (s *TodoistService) SetOAuthConfigForTest(conf *oauth2.Config) {
	s.conf = conf
}
