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
}

// bookLookup is the books surface (*books.Books) for linked resources.
type bookLookup interface {
	GetLibraryBookByID(
		ctx context.Context, userID string, bookID uuid.UUID,
	) (*booksv1.UserBook, error)
}

// feedItemLookup is the feeds surface (*feeds.Feeds) for linked resources.
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

// validateResourceLinks rejects any linked_book_id/linked_feed_item_id that
// doesn't resolve for userID before it is persisted.
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

// RecordItemProgress toggles an item's completion; ownership is enforced in
// the repository query.
func (s *LearningPathService) RecordItemProgress(
	ctx context.Context,
	userID string,
	itemID uuid.UUID,
	completed bool,
) error {
	return s.repo.RecordItemProgress(ctx, itemID, userID, completed)
}

// GetProgress is Get, named for the progress read.
func (s *LearningPathService) GetProgress(
	ctx context.Context,
	id uuid.UUID,
	userID string,
) (*models.LearningPath, error) {
	return s.Get(ctx, id, userID)
}
