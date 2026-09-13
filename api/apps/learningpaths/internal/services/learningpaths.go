package services

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/feeds"
	"tools.xdoubleu.com/apps/learningpaths/internal/models"
	booksv1 "tools.xdoubleu.com/gen/books/v1"
	iapp "tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/database"
)

// learningPathsStore is the storage surface LearningPathService needs. It is
// satisfied by repositories.LearningPathsRepository and by fakes in unit
// tests, so ownership rules can be tested without a database.
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
	// GetItemForUser is used by TodoistService.SendItem (issue #1475) to
	// build a task's content without pulling the whole path tree.
	GetItemForUser(
		ctx context.Context, itemID uuid.UUID, userID string,
	) (*models.ItemForTask, error)
}

// bookLookup is the surface LearningPathService needs from the books app to
// resolve/validate a resource's linked book. Satisfied by *books.Books
// (api/apps/books), narrowed here so unit tests can fake it without a
// database.
type bookLookup interface {
	GetLibraryBookByID(
		ctx context.Context, userID string, bookID uuid.UUID,
	) (*booksv1.UserBook, error)
}

// feedItemLookup is the feeds-side counterpart to bookLookup, satisfied by
// *feeds.Feeds (api/apps/feeds).
type feedItemLookup interface {
	GetItemByID(
		ctx context.Context, userID string, itemID uuid.UUID,
	) (*feeds.SharedItem, error)
}

type LearningPathService struct {
	repo  learningPathsStore
	books bookLookup
	feeds feedItemLookup
}

func (s *LearningPathService) List(
	ctx context.Context,
	userID string,
	limit int32,
	offset int32,
) ([]models.LearningPath, bool, error) {
	return s.repo.ListForUser(ctx, userID, limit, offset)
}

// Get returns a learning path owned by userID, with its modules/items/
// resources populated. Per-user scoping means there is no sharing concept —
// a path owned by someone else reports as not found, the same "404 on
// foreign ownership" rule used elsewhere for user-scoped lookups, rather
// than a 403 that would confirm the ID exists.
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

// resolveResourceLinks populates LinkedBook/LinkedFeedItem on every resource
// that carries a link ID, mutating resources in place. A link that no
// longer resolves (the book/feed item was removed, or — defensively — now
// belongs to someone else) is left unpopulated rather than failing the
// whole Get: a stale link is a display concern, not a reason to 404 an
// otherwise-valid learning path. Any other error (a real infrastructure
// failure) still propagates.
func (s *LearningPathService) resolveResourceLinks(
	ctx context.Context,
	userID string,
	resources []models.Resource,
) error {
	for i := range resources {
		if resources[i].LinkedBookID != nil {
			book, err := s.books.GetLibraryBookByID(ctx, userID, *resources[i].LinkedBookID)
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
			item, err := s.feeds.GetItemByID(ctx, userID, *resources[i].LinkedFeedItemID)
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

// validateResourceLinks confirms every linked_book_id/linked_feed_item_id a
// caller supplies on Create/Update actually resolves for userID before it is
// persisted — an unresolvable link (wrong ID, or an ID belonging to another
// user) is rejected up front rather than silently stored and only
// discovered missing on the next Get.
func (s *LearningPathService) validateResourceLinks(
	ctx context.Context,
	userID string,
	resources []models.Resource,
) error {
	for _, r := range resources {
		if r.LinkedBookID != nil {
			if _, err := s.books.GetLibraryBookByID(ctx, userID, *r.LinkedBookID); err != nil {
				if errors.Is(err, database.ErrResourceNotFound) {
					return &iapp.HTTPError{
						Status:  http.StatusBadRequest,
						Message: "linked_book_id does not resolve to a book in your library",
					}
				}
				return err
			}
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

func (s *LearningPathService) Create(
	ctx context.Context,
	userID string,
	lp models.LearningPath,
) (*models.LearningPath, error) {
	lp.UserID = userID

	if err := s.validateResourceLinks(ctx, userID, lp.Resources); err != nil {
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

	created.Modules = lp.Modules
	created.Resources = lp.Resources
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

	if err = s.validateResourceLinks(ctx, userID, lp.Resources); err != nil {
		return err
	}

	lp.UserID = existing.UserID
	if err = s.repo.Update(ctx, lp); err != nil {
		return err
	}
	if err = s.repo.ReplaceModules(ctx, lp.ID, lp.Modules); err != nil {
		return err
	}
	return s.repo.ReplaceResources(ctx, lp.ID, lp.Resources)
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
	return s.repo.Delete(ctx, id, userID)
}

// RecordItemProgress toggles a single item's completion flag. Ownership is
// enforced inside the repository query (joined through the item's module and
// path to userID), so there is no separate existence check here.
func (s *LearningPathService) RecordItemProgress(
	ctx context.Context,
	userID string,
	itemID uuid.UUID,
	completed bool,
) error {
	return s.repo.RecordItemProgress(ctx, itemID, userID, completed)
}

// GetProgress returns a learning path owned by userID with its
// modules/items/resources populated, for the caller to derive completion
// counts from. It is a thin alias over Get — reusing that method's ownership
// check and tree assembly — kept as its own name so the connect handler's
// intent (progress, not the full CRUD read) is clear at the call site.
func (s *LearningPathService) GetProgress(
	ctx context.Context,
	id uuid.UUID,
	userID string,
) (*models.LearningPath, error) {
	return s.Get(ctx, id, userID)
}
