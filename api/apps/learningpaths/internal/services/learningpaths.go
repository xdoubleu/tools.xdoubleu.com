package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/learningpaths/internal/models"
	iapp "tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/kit/database"
)

// LearningPathsStore is the storage surface LearningPathService needs.
type LearningPathsStore interface {
	ListForUser(
		ctx context.Context, userID string, limit, offset int32,
	) ([]models.LearningPath, bool, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.LearningPath, error)
	GetModules(ctx context.Context, id uuid.UUID) ([]models.Module, error)
	GetResources(ctx context.Context, id uuid.UUID) ([]models.Resource, error)
	Create(ctx context.Context, lp models.LearningPath) (*models.LearningPath, error)
	Update(ctx context.Context, lp models.LearningPath) error
	Delete(ctx context.Context, id uuid.UUID, userID string) error
	ReplaceModules(ctx context.Context, id uuid.UUID, modules []models.Module) error
	ReplaceResources(
		ctx context.Context,
		id uuid.UUID,
		resources []models.Resource,
	) error
	RecordItemProgress(
		ctx context.Context, itemID uuid.UUID, userID string, completed bool,
	) error
	GetItemForUser(
		ctx context.Context, itemID uuid.UUID, userID string,
	) (*models.ItemForTask, error)
	GetPathIDForItem(
		ctx context.Context, itemID uuid.UUID, userID string,
	) (uuid.UUID, error)
	SetItemTodoistTaskID(ctx context.Context, itemID uuid.UUID, taskID string) error
	SetTodoistProjectID(ctx context.Context, pathID uuid.UUID, projectID string) error
	ListForSync(ctx context.Context, userID string) ([]models.LearningPath, error)
	SetTodoistSectionID(ctx context.Context, pathID uuid.UUID, sectionID string) error
	GetUserTodoistProjectID(ctx context.Context, userID string) (string, error)
	SetUserTodoistProjectID(ctx context.Context, userID, projectID string) error
	SetPaused(ctx context.Context, id uuid.UUID, userID string, paused bool) error
}

// ErrExternalBookNotFound is returned by BookLookup.EnsureLibraryBook for a
// provider reference no provider resolves.
var ErrExternalBookNotFound = errors.New("external book not found")

// BookLookup is the books surface for linked resources. A book that is
// missing or not the user's reports database.ErrResourceNotFound.
type BookLookup interface {
	GetLibraryBook(
		ctx context.Context, userID string, bookID uuid.UUID,
	) (*models.LinkedBook, error)
	EnsureLibraryBook(
		ctx context.Context, userID, provider, providerID, shelf string,
	) (uuid.UUID, error)
}

// FeedItemLookup is the feeds surface for linked resources. A missing item
// reports database.ErrResourceNotFound.
type FeedItemLookup interface {
	GetFeedItem(
		ctx context.Context, userID string, itemID uuid.UUID,
	) (*models.LinkedFeedItem, error)
}

type LearningPathService struct {
	logger  *slog.Logger
	repo    LearningPathsStore
	books   BookLookup
	feeds   FeedItemLookup
	todoist *TodoistService
}

// logTodoistErr logs a failed best-effort Todoist call at Error level (so it
// reaches Sentry) without failing the request that triggered it.
func (s *LearningPathService) logTodoistErr(
	ctx context.Context, op, userID string, pathID uuid.UUID, err error,
) {
	if err == nil {
		return
	}
	s.logger.ErrorContext(ctx, "learningpaths: todoist "+op+" failed",
		"userID", userID, "pathID", pathID, "error", err)
}

func (s *LearningPathService) List(
	ctx context.Context,
	userID string,
	limit int32,
	offset int32,
) ([]models.LearningPath, bool, error) {
	return s.repo.ListForUser(ctx, userID, limit, offset)
}

// Get returns a learning path owned by userID with its tree populated. A
// foreign path reports not found, not forbidden.
func (s *LearningPathService) Get(
	ctx context.Context,
	id uuid.UUID,
	userID string,
) (*models.LearningPath, error) {
	lp, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if lp.UserID != userID {
		return nil, database.ErrResourceNotFound
	}

	modules, err := s.repo.GetModules(ctx, id)
	if err != nil {
		return nil, err
	}
	lp.Modules = modules

	if err = s.resolveItemLinks(ctx, userID, modules); err != nil {
		return nil, err
	}

	resources, err := s.repo.GetResources(ctx, id)
	if err != nil {
		return nil, err
	}
	if err = s.resolveResourceLinks(ctx, userID, resources); err != nil {
		return nil, err
	}
	lp.Resources = resources

	return lp, nil
}

// resolveItemLinks derives each book-linked item's Completed from its linked
// book and populates LinkedBook in place. A link that no longer resolves
// reads as incomplete rather than failing Get. A book at 100% progress is
// complete; below that it is not. Non-book items are untouched — their stored
// Completed stands.
func (s *LearningPathService) resolveItemLinks(
	ctx context.Context,
	userID string,
	modules []models.Module,
) error {
	for i := range modules {
		for j := range modules[i].Items {
			if modules[i].Items[j].LinkedBookID == nil {
				continue
			}
			book, err := s.books.GetLibraryBook(
				ctx,
				userID,
				*modules[i].Items[j].LinkedBookID,
			)
			if err != nil && !errors.Is(err, database.ErrResourceNotFound) {
				return err
			}
			if book == nil {
				modules[i].Items[j].Completed = false
				continue
			}
			modules[i].Items[j].LinkedBook = book
			modules[i].Items[j].Completed =
				book.ProgressPercent >= models.FullProgressPercent
		}
	}
	return nil
}

// resolveResourceLinks populates LinkedBook/LinkedFeedItem in place. A link
// that no longer resolves is left empty rather than failing Get; other
// errors propagate.
func (s *LearningPathService) resolveResourceLinks(
	ctx context.Context,
	userID string,
	resources []models.Resource,
) error {
	for i := range resources {
		if resources[i].LinkedBookID != nil {
			book, err := s.books.GetLibraryBook(
				ctx,
				userID,
				*resources[i].LinkedBookID,
			)
			if err != nil && !errors.Is(err, database.ErrResourceNotFound) {
				return err
			}
			if book != nil {
				resources[i].LinkedBook = book
			}
		}
		if resources[i].LinkedFeedItemID != nil {
			item, err := s.feeds.GetFeedItem(
				ctx,
				userID,
				*resources[i].LinkedFeedItemID,
			)
			if err != nil && !errors.Is(err, database.ErrResourceNotFound) {
				return err
			}
			if item != nil {
				resources[i].LinkedFeedItem = item
			}
		}
	}
	return nil
}

// validateResourceLinks rejects any linked_book_id/linked_feed_item_id that
// doesn't resolve for userID before it is persisted. Resource links and
// book-linked item links are validated the same way; item feed links don't
// exist, so items only validate books.
func (s *LearningPathService) validateResourceLinks(
	ctx context.Context,
	userID string,
	lp models.LearningPath,
) error {
	for _, m := range lp.Modules {
		for _, it := range m.Items {
			if err := s.validateBookLink(ctx, userID, it.LinkedBookID); err != nil {
				return err
			}
		}
	}
	for _, r := range lp.Resources {
		if err := s.validateBookLink(ctx, userID, r.LinkedBookID); err != nil {
			return err
		}
		if r.LinkedFeedItemID != nil {
			if _, err := s.feeds.GetFeedItem(ctx, userID, *r.LinkedFeedItemID); err != nil {
				if errors.Is(err, database.ErrResourceNotFound) {
					return iapp.NewBadRequestError(
						"linked_feed_item_id does not resolve to one of your feed items",
					)
				}
				return err
			}
		}
	}
	return nil
}

// validateBookLink returns nil when bookID is nil or resolves for userID,
// else a 400 (unknown) or the underlying error.
func (s *LearningPathService) validateBookLink(
	ctx context.Context,
	userID string,
	bookID *uuid.UUID,
) error {
	if bookID == nil {
		return nil
	}
	if _, err := s.books.GetLibraryBook(ctx, userID, *bookID); err != nil {
		if errors.Is(err, database.ErrResourceNotFound) {
			return iapp.NewBadRequestError(
				"linked_book_id does not resolve to a book in your library",
			)
		}
		return err
	}
	return nil
}

func (s *LearningPathService) Create(
	ctx context.Context,
	userID string,
	lp models.LearningPath,
) (*models.LearningPath, error) {
	lp.UserID = userID
	lp.ReminderSchedules = models.NormalizeReminderSchedules(lp.ReminderSchedules)

	if err := s.linkExternalBooks(ctx, userID, lp.Modules); err != nil {
		return nil, err
	}
	if err := s.validateResourceLinks(ctx, userID, lp); err != nil {
		return nil, err
	}

	created, err := s.repo.Create(ctx, lp)
	if err != nil {
		return nil, err
	}

	if err = s.repo.ReplaceModules(ctx, created.ID, lp.Modules); err != nil {
		return nil, err
	}
	if err = s.repo.ReplaceResources(ctx, created.ID, lp.Resources); err != nil {
		return nil, err
	}
	if err = s.resolveResourceLinks(ctx, userID, lp.Resources); err != nil {
		return nil, err
	}
	if err = s.resolveItemLinks(ctx, userID, lp.Modules); err != nil {
		return nil, err
	}

	created.Modules = lp.Modules
	created.Resources = lp.Resources

	// Todoist is a best-effort reminder outbox: a failure here must not fail
	// creating the path, and an absent connection is a graceful no-op.
	s.logTodoistErr(ctx, "sync", userID, created.ID, s.todoist.SyncUser(ctx, userID))
	return created, nil
}

func (s *LearningPathService) Update(
	ctx context.Context,
	userID string,
	lp models.LearningPath,
) error {
	existing, err := s.repo.GetByID(ctx, lp.ID)
	if err != nil {
		return err
	}
	if existing.UserID != userID {
		return database.ErrResourceNotFound
	}

	if err = s.linkExternalBooks(ctx, userID, lp.Modules); err != nil {
		return err
	}
	if err = s.validateResourceLinks(ctx, userID, lp); err != nil {
		return err
	}

	// Replacing modules reinserts every item, so their tasks would orphan:
	// clear them first and let the sync below recreate the next item's.
	// Best-effort, like every Todoist call.
	if oldModules, modErr := s.repo.GetModules(ctx, lp.ID); modErr == nil {
		s.logTodoistErr(ctx, "clear", userID, lp.ID,
			s.todoist.ClearTasks(ctx, userID, oldModules))
	}

	lp.UserID = existing.UserID
	lp.ReminderSchedules = models.NormalizeReminderSchedules(lp.ReminderSchedules)
	if err = s.repo.Update(ctx, lp); err != nil {
		return err
	}
	if err = s.repo.ReplaceModules(ctx, lp.ID, lp.Modules); err != nil {
		return err
	}
	if err = s.repo.ReplaceResources(ctx, lp.ID, lp.Resources); err != nil {
		return err
	}
	s.logTodoistErr(ctx, "sync", userID, lp.ID, s.todoist.SyncUser(ctx, userID))
	return nil
}

// linkExternalBooks resolves each item's ExternalBook into LinkedBookID,
// adding the book to the LearnShelf when userID doesn't own it yet. An
// explicit LinkedBookID wins.
func (s *LearningPathService) linkExternalBooks(
	ctx context.Context,
	userID string,
	modules []models.Module,
) error {
	for i := range modules {
		for j := range modules[i].Items {
			it := &modules[i].Items[j]
			if it.ExternalBook == nil || it.LinkedBookID != nil {
				continue
			}
			bookID, err := s.books.EnsureLibraryBook(
				ctx, userID,
				it.ExternalBook.Provider, it.ExternalBook.ProviderID,
				models.LearnShelf,
			)
			if errors.Is(err, ErrExternalBookNotFound) {
				return iapp.NewBadRequestError(fmt.Sprintf(
					"external_book %s/%s does not resolve to a book",
					it.ExternalBook.Provider, it.ExternalBook.ProviderID,
				))
			}
			if err != nil {
				return err
			}
			it.LinkedBookID = &bookID
		}
	}
	return nil
}

func (s *LearningPathService) Delete(
	ctx context.Context,
	id uuid.UUID,
	userID string,
) error {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if existing.UserID != userID {
		return database.ErrResourceNotFound
	}
	// Best-effort, like every Todoist call.
	if modules, modErr := s.repo.GetModules(ctx, id); modErr == nil {
		s.logTodoistErr(ctx, "delete", userID, id,
			s.todoist.DeletePath(ctx, userID, existing, modules))
	}
	if err = s.repo.Delete(ctx, id, userID); err != nil {
		return err
	}
	// The freed item types may unblock another path's next item.
	s.logTodoistErr(ctx, "sync", userID, id, s.todoist.SyncUser(ctx, userID))
	return nil
}

// RecordItemProgress toggles a non-book-linked item's completion; ownership is
// enforced in the repository query. Book-linked items derive completion from
// the book and can't be toggled — returning ErrResourceNotFound-equivalent
// would mislead, so reject with a 400. Todoist is then reconciled, since the
// toggle moves the path's next item.
func (s *LearningPathService) RecordItemProgress(
	ctx context.Context,
	userID string,
	itemID uuid.UUID,
	completed bool,
) error {
	item, err := s.repo.GetItemForUser(ctx, itemID, userID)
	if err != nil {
		return err
	}
	if item.Item.LinkedBookID != nil {
		return iapp.NewBadRequestError(
			"book-linked items complete automatically from your reading progress",
		)
	}
	if err = s.repo.RecordItemProgress(ctx, itemID, userID, completed); err != nil {
		return err
	}

	pathID, err := s.repo.GetPathIDForItem(ctx, itemID, userID)
	if err != nil {
		return err
	}
	s.logTodoistErr(ctx, "sync", userID, pathID, s.todoist.SyncUser(ctx, userID))
	return nil
}

// SetPaused pauses or resumes userID's path, then reconciles Todoist: pausing
// removes the path's task, resuming offers its next item again.
func (s *LearningPathService) SetPaused(
	ctx context.Context,
	id uuid.UUID,
	userID string,
	paused bool,
) error {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if existing.UserID != userID {
		return database.ErrResourceNotFound
	}
	if err = s.repo.SetPaused(ctx, id, userID, paused); err != nil {
		return err
	}
	s.logTodoistErr(ctx, "sync", userID, id, s.todoist.SyncUser(ctx, userID))
	return nil
}

// Progress derives userID's per-module and overall completion counts for a
// path; book-linked items count as complete once their book is.
func (s *LearningPathService) Progress(
	ctx context.Context,
	id uuid.UUID,
	userID string,
) (*models.Progress, error) {
	lp, err := s.Get(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	return models.ProgressOf(lp), nil
}
