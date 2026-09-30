package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/oauth2"

	"tools.xdoubleu.com/apps/learningpaths/internal/models"
	"tools.xdoubleu.com/apps/learningpaths/internal/repositories"
	"tools.xdoubleu.com/internal/database"
	sharedmodels "tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/oauthconn"
	"tools.xdoubleu.com/internal/todoist"
)

// ErrTodoistNotConnected is returned when the caller hasn't connected Todoist;
// callers map it to FailedPrecondition, not a 500.
var ErrTodoistNotConnected = oauthconn.ErrNotConnected

// todoistConnections is the oauth-connections surface TodoistService needs: a
// connected check plus per-user token lookup and connection mutation.
type todoistConnections interface {
	Status(ctx context.Context, userID string) (*sharedmodels.OAuthConnection, error)
	Upsert(
		ctx context.Context, userID string, provider sharedmodels.OAuthProvider,
		tok *oauth2.Token,
	) error
	Delete(
		ctx context.Context,
		userID string,
		provider sharedmodels.OAuthProvider,
	) error
	ForUser(userID string) repositories.UserScopedOAuthStore
}

// TodoistService connects a user's own Todoist account and mirrors learning
// path items to it as tasks, one-way. Strictly per-user; no admin bypass.
type TodoistService struct {
	oauthRepo     todoistConnections
	learningPaths learningPathsStore
	conf          *oauth2.Config
	state         *oauthconn.StateStore
	// newClient is a field so tests can substitute a mock Client.
	newClient func(oauthconn.TokenFunc) todoist.Client
	// resolveItems derives book-linked items' Completed before the active
	// module is picked; nil leaves the stored flags.
	resolveItems func(ctx context.Context, userID string, modules []models.Module) error
}

func NewTodoistService(
	oauthRepo todoistConnections,
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
		resolveItems:  nil,
	}
}

// AuthorizeURL issues a CSRF state for userID and returns the authorize URL.
// The callback leg is plain HTTP (routes.go).
func (s *TodoistService) AuthorizeURL(userID string) string {
	state := s.state.New(sharedmodels.OAuthProviderTodoist, userID)
	return s.conf.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

// HandleCallback consumes state, exchanges code, and stores the connection
// for sessionUserID, who must be the user the state was issued for: otherwise
// a victim following an attacker's authorize link would link their Todoist
// account to the attacker.
func (s *TodoistService) HandleCallback(
	ctx context.Context, sessionUserID, state, code string,
) (string, error) {
	provider, userID, ok := s.state.Consume(state)
	if !ok || provider != sharedmodels.OAuthProviderTodoist ||
		userID != sessionUserID {
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
	conn, err := s.oauthRepo.Status(ctx, userID)
	if errors.Is(err, database.ErrResourceNotFound) {
		return false, time.Time{}, nil
	}
	if err != nil {
		return false, time.Time{}, err
	}
	return true, conn.ConnectedAt, nil
}

// SyncPath reconciles userID's tasks to a strictly linear reminder pipeline:
// only the active module (the first with an incomplete item, in sort order)
// has tasks. On path creation that is the first module; completing module N
// removes its tasks and activates N+1. Idempotent — a task already completed
// or deleted in Todoist is ignored — and a graceful no-op when Todoist is
// disconnected. Completion is one-way: Todoist never flips a path item.
func (s *TodoistService) SyncPath(
	ctx context.Context, userID string, pathID uuid.UUID,
) error {
	connected, _, err := s.Status(ctx, userID)
	if err != nil {
		return err
	}
	if !connected {
		return nil
	}

	lp, err := s.learningPaths.GetByID(ctx, pathID)
	if err != nil {
		return err
	}
	if lp.UserID != userID {
		return database.ErrResourceNotFound
	}

	modules, err := s.learningPaths.GetModules(ctx, pathID)
	if err != nil {
		return err
	}
	if s.resolveItems != nil {
		if err = s.resolveItems(ctx, userID, modules); err != nil {
			return err
		}
	}

	active := s.activeModule(modules)
	client := s.userClient(userID)
	if err = s.clearNonActiveTasks(ctx, client, modules, active); err != nil {
		return err
	}
	return s.createActiveTasks(ctx, client, lp, modules, active)
}

// DeletePath removes every task of a path being deleted, then its project;
// a no-op when Todoist is disconnected.
func (s *TodoistService) DeletePath(
	ctx context.Context,
	userID string,
	lp *models.LearningPath,
	modules []models.Module,
) error {
	connected, _, err := s.Status(ctx, userID)
	if err != nil || !connected {
		return err
	}
	client := s.userClient(userID)
	if err = s.clearNonActiveTasks(ctx, client, modules, -1); err != nil {
		return err
	}
	if lp.TodoistProjectID == nil || *lp.TodoistProjectID == "" {
		return nil
	}
	return client.DeleteProject(ctx, *lp.TodoistProjectID)
}

// ClearTasks deletes every task modules' items carry; a no-op when Todoist
// is disconnected.
func (s *TodoistService) ClearTasks(
	ctx context.Context, userID string, modules []models.Module,
) error {
	connected, _, err := s.Status(ctx, userID)
	if err != nil || !connected {
		return err
	}
	return s.clearNonActiveTasks(ctx, s.userClient(userID), modules, -1)
}

func (s *TodoistService) userClient(userID string) todoist.Client {
	return s.newClient(oauthconn.NewTokenFunc(
		s.oauthRepo.ForUser(userID), sharedmodels.OAuthProviderTodoist, s.conf,
	))
}

// activeModule returns the index of the first module with an incomplete item,
// or -1 when every module is done (so no module should carry tasks).
func (s *TodoistService) activeModule(modules []models.Module) int {
	for i := range modules {
		for _, it := range modules[i].Items {
			if !it.Completed {
				return i
			}
		}
	}
	return -1
}

// clearNonActiveTasks removes (and forgets) the stored task of every item
// whose module is not active. Deleting a task Todoist no longer knows is not
// an error, so retries stay idempotent.
func (s *TodoistService) clearNonActiveTasks(
	ctx context.Context,
	client todoist.Client,
	modules []models.Module,
	active int,
) error {
	for i := range modules {
		if i == active {
			continue
		}
		if err := s.clearModuleTasks(ctx, client, modules[i]); err != nil {
			return err
		}
	}
	return nil
}

func (s *TodoistService) clearModuleTasks(
	ctx context.Context, client todoist.Client, module models.Module,
) error {
	for _, it := range module.Items {
		if it.TodoistTaskID == nil || *it.TodoistTaskID == "" {
			continue
		}
		if err := client.DeleteTask(ctx, *it.TodoistTaskID); err != nil {
			return err
		}
		if err := s.learningPaths.SetItemTodoistTaskID(ctx, it.ID, ""); err != nil {
			return err
		}
	}
	return nil
}

// createActiveTasks creates a task in the path's project for each item of
// the active module that does not already carry one, persisting the new task
// id on the item. When the project had to be (re)created, the module's
// existing tasks live elsewhere (the Inbox, or a deleted project), so they
// are replaced.
func (s *TodoistService) createActiveTasks(
	ctx context.Context,
	client todoist.Client,
	lp *models.LearningPath,
	modules []models.Module,
	active int,
) error {
	if active < 0 {
		return nil
	}
	projectID, created, err := s.ensureProject(ctx, client, lp)
	if err != nil {
		return err
	}
	items := modules[active].Items
	if created {
		if err = s.clearModuleTasks(ctx, client, modules[active]); err != nil {
			return err
		}
		for i := range items {
			items[i].TodoistTaskID = nil
		}
	}
	for _, it := range items {
		if it.TodoistTaskID != nil && *it.TodoistTaskID != "" {
			continue
		}
		taskID, createErr := client.CreateTask(ctx, it.Description, it.Due, projectID)
		if createErr != nil {
			return createErr
		}
		if err = s.learningPaths.SetItemTodoistTaskID(ctx, it.ID, taskID); err != nil {
			return err
		}
	}
	return nil
}

// ensureProject returns lp's Todoist project id, creating the project (named
// after the path) when it was never created or was deleted in Todoist.
func (s *TodoistService) ensureProject(
	ctx context.Context, client todoist.Client, lp *models.LearningPath,
) (string, bool, error) {
	if lp.TodoistProjectID != nil && *lp.TodoistProjectID != "" {
		exists, err := client.ProjectExists(ctx, *lp.TodoistProjectID)
		if err != nil {
			return "", false, err
		}
		if exists {
			return *lp.TodoistProjectID, false, nil
		}
	}
	projectID, err := client.CreateProject(ctx, lp.Title)
	if err != nil {
		return "", false, err
	}
	if err = s.learningPaths.SetTodoistProjectID(ctx, lp.ID, projectID); err != nil {
		return "", false, err
	}
	return projectID, true, nil
}

// SetOAuthConfigForTest overrides the Todoist OAuth2 config, e.g. to point
// TokenURL at an httptest server. Tests only.
func (s *TodoistService) SetOAuthConfigForTest(conf *oauth2.Config) {
	s.conf = conf
}
