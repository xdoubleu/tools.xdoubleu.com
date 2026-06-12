package services

import (
	"context"
	"net/http"

	"tools.xdoubleu.com/apps/shoppinglist/internal/repositories"
	iapp "tools.xdoubleu.com/internal/app"
)

type sharingRepo interface {
	ShareList(ctx context.Context, ownerID, targetUserID string, canEdit bool) error
	UnshareList(ctx context.Context, ownerID, targetUserID string) error
	ListShares(
		ctx context.Context,
		ownerID string,
	) ([]repositories.ShoppingListShare, error)
	GetListAccess(
		ctx context.Context,
		ownerID, viewerID string,
	) (canEdit, ok bool, err error)
	ListAccessibleOwners(
		ctx context.Context,
		viewerID string,
	) ([]repositories.ListOwner, error)
}

type SharingService struct {
	repo sharingRepo
}

// NewSharingService constructs a SharingService from any sharingRepo
// implementation, allowing injection of mocks in tests.
func NewSharingService(repo sharingRepo) *SharingService {
	return &SharingService{repo: repo}
}

func (s *SharingService) Share(
	ctx context.Context,
	ownerID, targetUserID string,
	canEdit bool,
) error {
	if targetUserID == "" || targetUserID == ownerID {
		return &iapp.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Invalid contact to share with",
		}
	}
	return s.repo.ShareList(ctx, ownerID, targetUserID, canEdit)
}

func (s *SharingService) Unshare(
	ctx context.Context,
	ownerID, targetUserID string,
) error {
	return s.repo.UnshareList(ctx, ownerID, targetUserID)
}

func (s *SharingService) ListShares(
	ctx context.Context,
	ownerID string,
) ([]repositories.ShoppingListShare, error) {
	return s.repo.ListShares(ctx, ownerID)
}

// AccessibleOwners returns the viewer's own list first, then lists shared with
// them.
func (s *SharingService) AccessibleOwners(
	ctx context.Context,
	viewerID string,
) ([]repositories.ListOwner, error) {
	shared, err := s.repo.ListAccessibleOwners(ctx, viewerID)
	if err != nil {
		return nil, err
	}
	owners := make([]repositories.ListOwner, 0, len(shared)+1)
	owners = append(owners, repositories.ListOwner{
		UserID:      viewerID,
		DisplayName: "",
		CanEdit:     true,
		IsSelf:      true,
	})
	return append(owners, shared...), nil
}

// ResolveOwner returns the effective owner ID a request should act on,
// enforcing access. An empty or self requestedOwnerID resolves to the viewer.
// Writes additionally require can_edit on a shared list.
func (s *SharingService) ResolveOwner(
	ctx context.Context,
	requestedOwnerID, viewerID string,
	write bool,
) (string, error) {
	if requestedOwnerID == "" || requestedOwnerID == viewerID {
		return viewerID, nil
	}

	canEdit, ok, err := s.repo.GetListAccess(ctx, requestedOwnerID, viewerID)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", &iapp.HTTPError{
			Status:  http.StatusForbidden,
			Message: "You do not have access to this shopping list",
		}
	}
	if write && !canEdit {
		return "", &iapp.HTTPError{
			Status:  http.StatusForbidden,
			Message: "You have read-only access to this shopping list",
		}
	}
	return requestedOwnerID, nil
}
