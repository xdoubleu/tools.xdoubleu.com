package repositories

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
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

// ShareList grants targetUserID access to ownerID's shopping list.
func (r *ShoppingRepository) ShareList(
	ctx context.Context,
	ownerID, targetUserID string,
	canEdit bool,
) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO shoppinglist.shoppinglist_access
			(owner_user_id, user_id, can_edit)
		VALUES ($1, $2, $3)
		ON CONFLICT (owner_user_id, user_id)
		DO UPDATE SET can_edit = EXCLUDED.can_edit`,
		ownerID, targetUserID, canEdit,
	)
	return err
}

func (r *ShoppingRepository) UnshareList(
	ctx context.Context,
	ownerID, targetUserID string,
) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM shoppinglist.shoppinglist_access
		WHERE owner_user_id = $1 AND user_id = $2`,
		ownerID, targetUserID,
	)
	return err
}

// ListShares returns the users ownerID shares their list with, resolving
// display names from the owner's contacts.
func (r *ShoppingRepository) ListShares(
	ctx context.Context,
	ownerID string,
) ([]ShoppingListShare, error) {
	rows, err := r.db.Query(ctx, `
		SELECT a.user_id, a.can_edit,
		       COALESCE(c.display_name, a.user_id) AS display_name
		FROM shoppinglist.shoppinglist_access a
		LEFT JOIN global.contacts c
		       ON c.owner_user_id = $1 AND c.contact_user_id = a.user_id
		WHERE a.owner_user_id = $1
		ORDER BY display_name`,
		ownerID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ShoppingListShare
	for rows.Next() {
		var s ShoppingListShare
		if err = rows.Scan(&s.UserID, &s.CanEdit, &s.DisplayName); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

// GetListAccess reports whether viewerID may act on ownerID's list and, if so,
// whether with edit rights.
func (r *ShoppingRepository) GetListAccess(
	ctx context.Context,
	ownerID, viewerID string,
) (bool, bool, error) {
	var canEdit bool
	err := r.db.QueryRow(ctx, `
		SELECT can_edit FROM shoppinglist.shoppinglist_access
		WHERE owner_user_id = $1 AND user_id = $2`,
		ownerID, viewerID,
	).Scan(&canEdit)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, false, nil
		}
		return false, false, err
	}
	return canEdit, true, nil
}

// ListAccessibleOwners returns the lists shared with viewerID (not including
// their own), resolving owner display names from the viewer's contacts.
func (r *ShoppingRepository) ListAccessibleOwners(
	ctx context.Context,
	viewerID string,
) ([]ListOwner, error) {
	rows, err := r.db.Query(ctx, `
		SELECT a.owner_user_id, a.can_edit,
		       COALESCE(c.display_name, a.owner_user_id) AS display_name
		FROM shoppinglist.shoppinglist_access a
		LEFT JOIN global.contacts c
		       ON c.owner_user_id = $1 AND c.contact_user_id = a.owner_user_id
		WHERE a.user_id = $1
		ORDER BY display_name`,
		viewerID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ListOwner
	for rows.Next() {
		o := ListOwner{IsSelf: false} //nolint:exhaustruct // scanned below
		if err = rows.Scan(&o.UserID, &o.CanEdit, &o.DisplayName); err != nil {
			return nil, err
		}
		result = append(result, o)
	}
	return result, rows.Err()
}
