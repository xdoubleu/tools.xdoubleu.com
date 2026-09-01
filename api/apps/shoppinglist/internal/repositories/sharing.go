package repositories

import (
	"context"

	"tools.xdoubleu.com/internal/sharing"
)

// ShoppingListShare is a user the owner shares their list with.
type ShoppingListShare struct {
	UserID      string
	CanEdit     bool
	DisplayName string
}

// ListOwner is a list the viewer can act on: their own or one shared with them.
type ListOwner struct {
	UserID      string
	DisplayName string
	CanEdit     bool
	IsSelf      bool
}

// sharingAccess is shoppinglist's use of the owner/user/can_edit access
// pattern shared with recipes' recipe-book sharing — see
// internal/sharing.Repository.
func (r *ShoppingRepository) sharingAccess() *sharing.Repository {
	return sharing.NewRepository(r.db, "shoppinglist", "shoppinglist_access")
}

// ShareList grants targetUserID access to ownerID's shopping list.
func (r *ShoppingRepository) ShareList(
	ctx context.Context,
	ownerID, targetUserID string,
	canEdit bool,
) error {
	return r.sharingAccess().Share(ctx, ownerID, targetUserID, canEdit)
}

func (r *ShoppingRepository) UnshareList(
	ctx context.Context,
	ownerID, targetUserID string,
) error {
	return r.sharingAccess().Unshare(ctx, ownerID, targetUserID)
}

// ListShares returns the users ownerID shares their list with, resolving
// display names from the owner's contacts.
func (r *ShoppingRepository) ListShares(
	ctx context.Context,
	ownerID string,
) ([]ShoppingListShare, error) {
	shares, err := r.sharingAccess().ListShares(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	result := make([]ShoppingListShare, len(shares))
	for i, s := range shares {
		result[i] = ShoppingListShare{
			UserID:      s.UserID,
			CanEdit:     s.CanEdit,
			DisplayName: s.DisplayName,
		}
	}
	return result, nil
}

// GetListAccess reports whether viewerID may act on ownerID's list and, if so,
// whether with edit rights.
func (r *ShoppingRepository) GetListAccess(
	ctx context.Context,
	ownerID, viewerID string,
) (bool, bool, error) {
	return r.sharingAccess().GetAccess(ctx, ownerID, viewerID)
}

// ListAccessibleOwners returns the lists shared with viewerID (not including
// their own), resolving owner display names from the viewer's contacts.
func (r *ShoppingRepository) ListAccessibleOwners(
	ctx context.Context,
	viewerID string,
) ([]ListOwner, error) {
	owners, err := r.sharingAccess().ListAccessibleOwners(ctx, viewerID)
	if err != nil {
		return nil, err
	}
	result := make([]ListOwner, len(owners))
	for i, o := range owners {
		result[i] = ListOwner{
			UserID:      o.UserID,
			DisplayName: o.DisplayName,
			CanEdit:     o.CanEdit,
			IsSelf:      false,
		}
	}
	return result, nil
}
