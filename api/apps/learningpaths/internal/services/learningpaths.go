package services

import (
	"context"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/learningpaths/internal/models"
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
}

type LearningPathService struct {
	repo learningPathsStore
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
	lp.Resources = resources

	return lp, nil
}

func (s *LearningPathService) Create(
	ctx context.Context,
	userID string,
	lp models.LearningPath,
) (*models.LearningPath, error) {
	lp.UserID = userID

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
