package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/books"
	"tools.xdoubleu.com/apps/feeds"
	"tools.xdoubleu.com/apps/learningpaths/internal/models"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
	iapp "tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/database"
)

// learningPathsStore is the storage surface LearningPathService needs.
type learningPathsStore interface {
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
}

// bookLookup is the books surface (*books.Books) for linked resources.
type bookLookup interface {
	GetLibraryBookByID(
		ctx context.Context, userID string, bookID uuid.UUID,
	) (*booksv1.UserBook, error)
	EnsureLibraryBook(
		ctx context.Context, userID, provider, providerID, shelf string,
	) (uuid.UUID, error)
}

// feedItemLookup is the feeds surface (*feeds.Feeds) for linked resources.
type feedItemLookup interface {
	GetItemByID(
		ctx context.Context, userID string, itemID uuid.UUID,
	) (*feeds.SharedItem, error)
}

type LearningPathService struct {
	logger  *slog.Logger
	repo    learningPathsStore
	books   bookLookup
	feeds   feedItemLookup
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
			book, err := s.books.GetLibraryBookByID(
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
			modules[i].Items[j].LinkedBook = &models.LinkedBook{
				Title:           book.Book.GetTitle(),
				Status:          book.Status,
				ProgressPercent: int(book.ProgressPercent),
				CoverURL:        book.Book.GetCoverUrl(),
			}
			modules[i].Items[j].Completed =
				book.ProgressPercent >= int32(models.FullProgressPercent)
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
			book, err := s.books.GetLibraryBookByID(
				ctx,
				userID,
				*resources[i].LinkedBookID,
			)
			if err != nil && !errors.Is(err, database.ErrResourceNotFound) {
				return err
			}
			if book != nil {
				resources[i].LinkedBook = &models.LinkedBook{
					Title:           book.Book.GetTitle(),
					Status:          book.Status,
					ProgressPercent: int(book.ProgressPercent),
					CoverURL:        book.Book.GetCoverUrl(),
				}
			}
		}
		if resources[i].LinkedFeedItemID != nil {
			item, err := s.feeds.GetItemByID(
				ctx,
				userID,
				*resources[i].LinkedFeedItemID,
			)
			if err != nil && !errors.Is(err, database.ErrResourceNotFound) {
				return err
			}
			if item != nil {
				resources[i].LinkedFeedItem = &models.LinkedFeedItem{
					Title:      item.Title,
					SourceURL:  item.SourceURL,
					Read:       item.Read,
					Bookmarked: item.Bookmarked,
				}
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
			if _, err := s.feeds.GetItemByID(ctx, userID, *r.LinkedFeedItemID); err != nil {
				if errors.Is(err, database.ErrResourceNotFound) {
					return &iapp.HTTPError{
						Status:  http.StatusBadRequest,
						Message: "linked_feed_item_id does not resolve to one of your feed items",
					}
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
	if _, err := s.books.GetLibraryBookByID(ctx, userID, *bookID); err != nil {
		if errors.Is(err, database.ErrResourceNotFound) {
			return &iapp.HTTPError{
				Status:  http.StatusBadRequest,
				Message: "linked_book_id does not resolve to a book in your library",
			}
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

	// Activate the first module's items as Todoist tasks on path creation.
	// Todoist is a best-effort reminder outbox: a failure here must not fail
	// creating the path, and an absent connection is a graceful no-op.
	s.logTodoistErr(ctx, "sync", userID, created.ID,
		s.todoist.SyncPath(ctx, userID, created.ID))
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
	// clear them first and let the sync below recreate the active module's.
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
	s.logTodoistErr(ctx, "sync", userID, lp.ID, s.todoist.SyncPath(ctx, userID, lp.ID))
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
			if errors.Is(err, books.ErrExternalBookNotFound) {
				return &iapp.HTTPError{
					Status: http.StatusBadRequest,
					Message: fmt.Sprintf(
						"external_book %s/%s does not resolve to a book",
						it.ExternalBook.Provider, it.ExternalBook.ProviderID,
					),
				}
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
	return s.repo.Delete(ctx, id, userID)
}

// RecordItemProgress toggles a non-book-linked item's completion; ownership is
// enforced in the repository query. Book-linked items derive completion from
// the book and can't be toggled — returning ErrResourceNotFound-equivalent
// would mislead, so reject with a 400. When a non-book item completes, the
// Todoist pipeline is reconciled (module N done → its tasks removed, module
// N+1 activated).
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
		return &iapp.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "book-linked items complete automatically from your reading progress",
		}
	}
	if err = s.repo.RecordItemProgress(ctx, itemID, userID, completed); err != nil {
		return err
	}
	if !completed {
		return nil
	}

	pathID, err := s.repo.GetPathIDForItem(ctx, itemID, userID)
	if err != nil {
		return err
	}
	s.logTodoistErr(ctx, "sync", userID, pathID, s.todoist.SyncPath(ctx, userID, pathID))
	return nil
}

// GetProgress is Get, named for the progress read.
func (s *LearningPathService) GetProgress(
	ctx context.Context,
	id uuid.UUID,
	userID string,
) (*models.LearningPath, error) {
	return s.Get(ctx, id, userID)
}
