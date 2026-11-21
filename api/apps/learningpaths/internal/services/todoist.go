package services

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/oauth2"

	"tools.xdoubleu.com/apps/learningpaths/internal/models"
	"tools.xdoubleu.com/apps/learningpaths/internal/repositories"
	"tools.xdoubleu.com/apps/learningpaths/internal/todoist"
	"tools.xdoubleu.com/internal/kit/database"
	sharedmodels "tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/platform/oauthconn"
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
		tok *oauth2.Token, requestedScope string,
	) error
	Delete(
		ctx context.Context,
		userID string,
		provider sharedmodels.OAuthProvider,
	) error
	ForUser(userID string) repositories.UserScopedOAuthStore
	TodoistUserIDs(ctx context.Context) ([]string, error)
}

// TodoistService connects a user's own Todoist account and mirrors learning
// path items to it as tasks in one shared project, one-way. Strictly
// per-user; no admin bypass.
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
	// syncMu serializes syncs so a request and the resync job never both
	// create a path's task.
	syncMu sync.Mutex
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
		syncMu:        sync.Mutex{},
	}
}

// AuthorizeURL issues a CSRF state for userID and returns the authorize URL.
// The callback leg is plain HTTP (routes.go).
func (s *TodoistService) AuthorizeURL(userID string) string {
	state := s.state.New(sharedmodels.OAuthProviderTodoist, userID)
	return s.conf.AuthCodeURL(
		state, oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("scope", todoist.ScopeParam(s.conf.Scopes)),
	)
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
		todoist.ScopeParam(s.conf.Scopes),
	); err != nil {
		return "", err
	}

	return userID, nil
}

// Disconnect removes userID's stored Todoist connection, if any.
func (s *TodoistService) Disconnect(ctx context.Context, userID string) error {
	return s.oauthRepo.Delete(ctx, userID, sharedmodels.OAuthProviderTodoist)
}

// ConnectionStatus is userID's Todoist connection state.
type ConnectionStatus struct {
	Connected   bool
	ConnectedAt time.Time
	// NeedsReconnect: connected, but authorized with fewer scopes than the
	// reminder pipeline needs, so syncs skip it until the user reconnects.
	NeedsReconnect bool
	RequestedScope string
}

// Status reports userID's connection state; "not connected" is not an error.
func (s *TodoistService) Status(
	ctx context.Context, userID string,
) (ConnectionStatus, error) {
	conn, err := s.oauthRepo.Status(ctx, userID)
	if errors.Is(err, database.ErrResourceNotFound) {
		return ConnectionStatus{}, nil //nolint:exhaustruct // zero = disconnected
	}
	if err != nil {
		return ConnectionStatus{}, err
	}
	return ConnectionStatus{
		Connected:      true,
		ConnectedAt:    conn.ConnectedAt,
		NeedsReconnect: oauthconn.ScopesAreStale(conn, todoist.RequiredScopes()),
		RequestedScope: conn.RequestedScope,
	}, nil
}

// syncable reports whether userID's connection can run the reminder pipeline.
func (s *TodoistService) syncable(ctx context.Context, userID string) (bool, error) {
	st, err := s.Status(ctx, userID)
	return st.Connected && !st.NeedsReconnect, err
}

// ProjectName is the user's shared Todoist project holding every path.
const ProjectName = "Learning paths"

// SyncState is a path's Todoist state, for diagnosing reminders: the items
// of the active module plus any other item still carrying a task.
type SyncState struct {
	Status       ConnectionStatus
	ProjectID    string
	ActiveModule string
	Paused       bool
	Items        []models.Item
}

// SyncState reads userID's Todoist state for pathID without calling Todoist.
func (s *TodoistService) SyncState(
	ctx context.Context, userID string, pathID uuid.UUID,
) (*SyncState, error) {
	st, err := s.Status(ctx, userID)
	if err != nil {
		return nil, err
	}
	lp, err := s.learningPaths.GetByID(ctx, pathID)
	if err != nil {
		return nil, err
	}
	if lp.UserID != userID {
		return nil, database.ErrResourceNotFound
	}
	modules, err := s.loadModules(ctx, userID, pathID)
	if err != nil {
		return nil, err
	}
	projectID, err := s.learningPaths.GetUserTodoistProjectID(ctx, userID)
	if err != nil {
		return nil, err
	}

	//nolint:exhaustruct // filled below
	out := &SyncState{Status: st, ProjectID: projectID, Paused: lp.Paused}
	active := -1
	if !lp.Paused {
		active = activeModule(modules)
	}
	if active >= 0 {
		out.ActiveModule = modules[active].Title
	}
	for i, m := range modules {
		for _, it := range m.Items {
			if i == active || hasTask(it) {
				out.Items = append(out.Items, it)
			}
		}
	}
	return out, nil
}

// pathPlan is one path with its modules, loaded for a sync.
type pathPlan struct {
	lp      *models.LearningPath
	modules []models.Module
}

// SyncUser reconciles userID's Todoist project with every path: each unpaused
// path offers its next incomplete item, and at most one task per item type is
// open across paths (never two books at once). A path holding a type keeps
// it; free types go to the oldest path. Other tasks are removed. Idempotent,
// a no-op when disconnected, and one-way: Todoist never flips an item.
func (s *TodoistService) SyncUser(ctx context.Context, userID string) error {
	s.syncMu.Lock()
	defer s.syncMu.Unlock()

	connected, err := s.syncable(ctx, userID)
	if err != nil || !connected {
		return err
	}

	paths, err := s.learningPaths.ListForSync(ctx, userID)
	if err != nil {
		return err
	}
	plans := make([]pathPlan, len(paths))
	for i := range paths {
		modules, loadErr := s.loadModules(ctx, userID, paths[i].ID)
		if loadErr != nil {
			return loadErr
		}
		plans[i] = pathPlan{lp: &paths[i], modules: modules}
	}

	client := s.userClient(userID)
	if err = s.dropLegacyProjects(ctx, client, plans); err != nil {
		return err
	}
	next := selectNextItems(plans)
	if err = s.clearUnselected(ctx, client, plans, next); err != nil {
		return err
	}
	return s.createSelected(ctx, client, userID, plans, next)
}

// ResyncAll runs SyncUser for every connected user, continuing past one
// user's failure.
func (s *TodoistService) ResyncAll(ctx context.Context) error {
	userIDs, err := s.oauthRepo.TodoistUserIDs(ctx)
	if err != nil {
		return err
	}
	errs := make([]error, 0, len(userIDs))
	for _, userID := range userIDs {
		if syncErr := s.SyncUser(ctx, userID); syncErr != nil {
			errs = append(errs, fmt.Errorf("user %s: %w", userID, syncErr))
		}
	}
	return errors.Join(errs...)
}

// DeletePath removes every task and the section of a path being deleted; a
// no-op when Todoist is disconnected.
func (s *TodoistService) DeletePath(
	ctx context.Context,
	userID string,
	lp *models.LearningPath,
	modules []models.Module,
) error {
	connected, err := s.syncable(ctx, userID)
	if err != nil || !connected {
		return err
	}
	client := s.userClient(userID)
	if err = s.clearTasks(ctx, client, modules); err != nil {
		return err
	}
	if lp.TodoistSectionID != nil && *lp.TodoistSectionID != "" {
		if err = client.DeleteSection(ctx, *lp.TodoistSectionID); err != nil {
			return err
		}
	}
	if lp.TodoistProjectID != nil && *lp.TodoistProjectID != "" {
		return client.DeleteProject(ctx, *lp.TodoistProjectID)
	}
	return nil
}

// ClearTasks deletes every task modules' items carry; a no-op when Todoist
// is disconnected.
func (s *TodoistService) ClearTasks(
	ctx context.Context, userID string, modules []models.Module,
) error {
	connected, err := s.syncable(ctx, userID)
	if err != nil || !connected {
		return err
	}
	return s.clearTasks(ctx, s.userClient(userID), modules)
}

func (s *TodoistService) userClient(userID string) todoist.Client {
	return s.newClient(oauthconn.NewTokenFunc(
		s.oauthRepo.ForUser(userID), sharedmodels.OAuthProviderTodoist, s.conf,
	))
}

func (s *TodoistService) loadModules(
	ctx context.Context, userID string, pathID uuid.UUID,
) ([]models.Module, error) {
	modules, err := s.learningPaths.GetModules(ctx, pathID)
	if err != nil {
		return nil, err
	}
	if s.resolveItems != nil {
		if err = s.resolveItems(ctx, userID, modules); err != nil {
			return nil, err
		}
	}
	return modules, nil
}

// activeModule returns the index of the first module with an incomplete item,
// or -1 when every module is done.
func activeModule(modules []models.Module) int {
	for i := range modules {
		for _, it := range modules[i].Items {
			if !it.Completed {
				return i
			}
		}
	}
	return -1
}

// pathNextItem is a path's next item, or nil when paused or done: the first
// incomplete item of the active module, unless a book in that module is
// already being read, in which case the furthest-along one.
func pathNextItem(p pathPlan) *models.Item {
	if p.lp.Paused {
		return nil
	}
	active := activeModule(p.modules)
	if active < 0 {
		return nil
	}
	items := p.modules[active].Items
	var next, started *models.Item
	for j := range items {
		it := &items[j]
		if it.Completed {
			continue
		}
		if next == nil {
			next = it
		}
		if bookProgress(*it) > 0 &&
			(started == nil || bookProgress(*it) > bookProgress(*started)) {
			started = it
		}
	}
	if started != nil {
		return started
	}
	return next
}

// bookProgress is a linked book's reading progress, 0 when not book-linked.
func bookProgress(it models.Item) int {
	if it.LinkedBook == nil {
		return 0
	}
	return it.LinkedBook.ProgressPercent
}

func hasTask(it models.Item) bool {
	return it.TodoistTaskID != nil && *it.TodoistTaskID != ""
}

// selectNextItems picks, per item type, the one path whose next item gets a
// task: the path already holding a task of that type, else the oldest.
func selectNextItems(plans []pathPlan) map[uuid.UUID]bool {
	selected := map[uuid.UUID]bool{}
	taken := map[string]bool{}
	for _, holders := range []bool{true, false} {
		for i := range plans {
			it := pathNextItem(plans[i])
			if it == nil || hasTask(*it) != holders {
				continue
			}
			itemType := models.NormalizeItemType(it.Type)
			if taken[itemType] {
				continue
			}
			taken[itemType] = true
			selected[it.ID] = true
		}
	}
	return selected
}

// dropLegacyProjects deletes each path's pre-shared-project Todoist project,
// taking its tasks with it, so the next steps recreate them in the shared one.
func (s *TodoistService) dropLegacyProjects(
	ctx context.Context, client todoist.Client, plans []pathPlan,
) error {
	for _, p := range plans {
		if p.lp.TodoistProjectID == nil || *p.lp.TodoistProjectID == "" {
			continue
		}
		if err := client.DeleteProject(ctx, *p.lp.TodoistProjectID); err != nil {
			return err
		}
		if err := s.forgetTasks(ctx, p.modules); err != nil {
			return err
		}
		if err := s.learningPaths.SetTodoistProjectID(ctx, p.lp.ID, ""); err != nil {
			return err
		}
		p.lp.TodoistProjectID = nil
	}
	return nil
}

// forgetTasks clears stored task ids without calling Todoist.
func (s *TodoistService) forgetTasks(
	ctx context.Context, modules []models.Module,
) error {
	for i := range modules {
		for j := range modules[i].Items {
			it := &modules[i].Items[j]
			if !hasTask(*it) {
				continue
			}
			if err := s.learningPaths.SetItemTodoistTaskID(ctx, it.ID, ""); err != nil {
				return err
			}
			it.TodoistTaskID = nil
		}
	}
	return nil
}

// clearUnselected removes (and forgets) every task not of a selected item.
func (s *TodoistService) clearUnselected(
	ctx context.Context,
	client todoist.Client,
	plans []pathPlan,
	selected map[uuid.UUID]bool,
) error {
	for _, p := range plans {
		for i := range p.modules {
			for j := range p.modules[i].Items {
				it := &p.modules[i].Items[j]
				if selected[it.ID] {
					continue
				}
				if err := s.deleteTask(ctx, client, it); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// clearTasks removes (and forgets) every task modules' items carry.
func (s *TodoistService) clearTasks(
	ctx context.Context, client todoist.Client, modules []models.Module,
) error {
	for i := range modules {
		for j := range modules[i].Items {
			if err := s.deleteTask(ctx, client, &modules[i].Items[j]); err != nil {
				return err
			}
		}
	}
	return nil
}

// deleteTask removes it's task, if any. Deleting a task Todoist no longer
// knows is not an error, so retries stay idempotent.
func (s *TodoistService) deleteTask(
	ctx context.Context, client todoist.Client, it *models.Item,
) error {
	if !hasTask(*it) {
		return nil
	}
	if err := client.DeleteTask(ctx, *it.TodoistTaskID); err != nil {
		return err
	}
	if err := s.learningPaths.SetItemTodoistTaskID(ctx, it.ID, ""); err != nil {
		return err
	}
	it.TodoistTaskID = nil
	return nil
}

// createSelected creates a task, in its path's section of the shared
// project, for each selected item without one. When the project had to be
// (re)created, existing tasks live in a deleted project and are replaced.
func (s *TodoistService) createSelected(
	ctx context.Context,
	client todoist.Client,
	userID string,
	plans []pathPlan,
	selected map[uuid.UUID]bool,
) error {
	if len(selected) == 0 {
		return nil
	}
	projectID, created, err := s.ensureProject(ctx, client, userID)
	if err != nil {
		return err
	}
	for _, p := range plans {
		it := pathNextItem(p)
		if it == nil || !selected[it.ID] {
			continue
		}
		if created {
			if err = s.forgetTasks(ctx, p.modules); err != nil {
				return err
			}
		}
		if hasTask(*it) {
			continue
		}
		sectionID, sectionErr := s.ensureSection(ctx, client, p.lp, projectID, created)
		if sectionErr != nil {
			return sectionErr
		}
		taskID, createErr := client.CreateTask(
			ctx, it.Description, p.lp.ItemDue(*it), projectID, sectionID,
		)
		if createErr != nil {
			return createErr
		}
		if err = s.learningPaths.SetItemTodoistTaskID(ctx, it.ID, taskID); err != nil {
			return err
		}
	}
	return nil
}

// ensureProject returns userID's shared project id, creating the project
// when it was never created or was deleted in Todoist.
func (s *TodoistService) ensureProject(
	ctx context.Context, client todoist.Client, userID string,
) (string, bool, error) {
	projectID, err := s.learningPaths.GetUserTodoistProjectID(ctx, userID)
	if err != nil {
		return "", false, err
	}
	if projectID != "" {
		exists, existsErr := client.ProjectExists(ctx, projectID)
		if existsErr != nil {
			return "", false, existsErr
		}
		if exists {
			return projectID, false, nil
		}
	}
	projectID, err = client.CreateProject(ctx, ProjectName)
	if err != nil {
		return "", false, err
	}
	if err = s.learningPaths.SetUserTodoistProjectID(ctx, userID, projectID); err != nil {
		return "", false, err
	}
	return projectID, true, nil
}

// ensureSection returns lp's section id, creating the section (named after
// the path) when missing; projectCreated means any stored one is gone.
func (s *TodoistService) ensureSection(
	ctx context.Context,
	client todoist.Client,
	lp *models.LearningPath,
	projectID string,
	projectCreated bool,
) (string, error) {
	if !projectCreated && lp.TodoistSectionID != nil && *lp.TodoistSectionID != "" {
		exists, err := client.SectionExists(ctx, *lp.TodoistSectionID)
		if err != nil {
			return "", err
		}
		if exists {
			return *lp.TodoistSectionID, nil
		}
	}
	sectionID, err := client.CreateSection(ctx, projectID, lp.Title)
	if err != nil {
		return "", err
	}
	if err = s.learningPaths.SetTodoistSectionID(ctx, lp.ID, sectionID); err != nil {
		return "", err
	}
	lp.TodoistSectionID = &sectionID
	return sectionID, nil
}

// SetOAuthConfigForTest overrides the Todoist OAuth2 config, e.g. to point
// TokenURL at an httptest server. Tests only.
func (s *TodoistService) SetOAuthConfigForTest(conf *oauth2.Config) {
	s.conf = conf
}
