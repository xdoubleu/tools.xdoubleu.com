// Package sharing holds logic shared by recipes/mealplans/shoppinglist's
// "share a whole owned resource with another user" features: a validation
// rule every one of those RPCs must enforce, plus a generic repository for
// the owner/user/can_edit access-grant pattern that recipes' recipe book and
// shoppinglist's shopping list both use verbatim (see Repository's doc
// comment). mealplans shares per-plan rather than per-owner, so it only
// reuses the validation half.
package sharing

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5"

	"tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/database/postgres"
)

// ValidateShareTarget enforces the one rule every share RPC in this repo
// applies before writing a grant: the target must be specified and must not
// be the owner sharing with themselves. Call this at the top of every
// Share/SharePlan-style service method — recipes.RecipeService.ShareBook and
// shoppinglist's SharingService.Share already did this inline; mealplans'
// PlanService.Share did not, which let a caller grant an empty or
// self-referencing share (issue #1349).
func ValidateShareTarget(ownerID, targetUserID string) error {
	if targetUserID == "" || targetUserID == ownerID {
		return &app.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Invalid contact to share with",
		}
	}
	return nil
}

// Share is a user a resource owner has granted access to. UserID/CanEdit come
// from the access table; DisplayName resolves via the owner's contacts.
type Share struct {
	UserID      string
	CanEdit     bool
	DisplayName string
}

// Owner is a resource the viewer can act on because it was shared with them.
type Owner struct {
	UserID      string
	DisplayName string
	CanEdit     bool
}

// Repository implements the owner/user/can_edit "share this whole resource
// with a user" pattern used identically by shoppinglist
// (shoppinglist.shoppinglist_access) and recipes (recipes.recipebook_access):
// one row per (owner, grantee) pair, granting the grantee access to
// everything the owner owns in that app. schema and table are trusted Go
// string literals supplied by the caller at construction — never derived
// from request input — so NewRepository builds the five query strings once,
// up front; every caller-supplied value still goes through parameterized
// placeholders, never string interpolation.
type Repository struct {
	db postgres.DB

	shareQuery         string
	unshareQuery       string
	listSharesQuery    string
	getAccessQuery     string
	accessibleOwnQuery string
}

// NewRepository builds a Repository over the access table `<schema>.<table>`,
// which must have the shape `(owner_user_id TEXT, user_id TEXT, can_edit
// BOOL, PRIMARY KEY (owner_user_id, user_id))`. schema/table are trusted Go
// string literals from the caller, never request input, so building the
// query text with fmt.Sprintf here is safe.
func NewRepository(db postgres.DB, schema, table string) *Repository {
	qualified := schema + "." + table
	return &Repository{
		db: db,
		shareQuery: fmt.Sprintf(`
			INSERT INTO %s (owner_user_id, user_id, can_edit)
			VALUES ($1, $2, $3)
			ON CONFLICT (owner_user_id, user_id)
			DO UPDATE SET can_edit = EXCLUDED.can_edit`, qualified),
		unshareQuery: fmt.Sprintf(
			`DELETE FROM %s WHERE owner_user_id = $1 AND user_id = $2`, qualified,
		),
		listSharesQuery: fmt.Sprintf(`
			SELECT a.user_id, a.can_edit,
			       COALESCE(c.display_name, a.user_id) AS display_name
			FROM %s a
			LEFT JOIN global.contacts c
			       ON c.owner_user_id = $1 AND c.contact_user_id = a.user_id
			WHERE a.owner_user_id = $1
			ORDER BY display_name`, qualified),
		getAccessQuery: fmt.Sprintf(
			`SELECT can_edit FROM %s WHERE owner_user_id = $1 AND user_id = $2`,
			qualified,
		),
		accessibleOwnQuery: fmt.Sprintf(`
			SELECT a.owner_user_id, a.can_edit,
			       COALESCE(c.display_name, a.owner_user_id) AS display_name
			FROM %s a
			LEFT JOIN global.contacts c
			       ON c.owner_user_id = $1 AND c.contact_user_id = a.owner_user_id
			WHERE a.user_id = $1
			ORDER BY display_name`, qualified),
	}
}

// Share grants targetUserID access to ownerID's resource, replacing any
// existing grant's can_edit flag.
func (r *Repository) Share(
	ctx context.Context,
	ownerID, targetUserID string,
	canEdit bool,
) error {
	_, err := r.db.Exec(ctx, r.shareQuery, ownerID, targetUserID, canEdit)
	return err
}

// Unshare revokes targetUserID's access to ownerID's resource.
func (r *Repository) Unshare(ctx context.Context, ownerID, targetUserID string) error {
	_, err := r.db.Exec(ctx, r.unshareQuery, ownerID, targetUserID)
	return err
}

// ListShares returns the users ownerID shares their resource with, resolving
// display names from the owner's contacts.
func (r *Repository) ListShares(ctx context.Context, ownerID string) ([]Share, error) {
	rows, err := r.db.Query(ctx, r.listSharesQuery, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Share
	for rows.Next() {
		var s Share
		if err = rows.Scan(&s.UserID, &s.CanEdit, &s.DisplayName); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

// GetAccess reports whether viewerID may act on ownerID's resource and, if
// so, whether with edit rights.
func (r *Repository) GetAccess(
	ctx context.Context,
	ownerID, viewerID string,
) (bool, bool, error) {
	var canEdit bool
	err := r.db.QueryRow(ctx, r.getAccessQuery, ownerID, viewerID).Scan(&canEdit)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, false, nil
		}
		return false, false, err
	}
	return canEdit, true, nil
}

// ListAccessibleOwners returns the resources shared with viewerID (not
// including their own), resolving owner display names from the viewer's
// contacts.
func (r *Repository) ListAccessibleOwners(
	ctx context.Context,
	viewerID string,
) ([]Owner, error) {
	rows, err := r.db.Query(ctx, r.accessibleOwnQuery, viewerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Owner
	for rows.Next() {
		var o Owner
		if err = rows.Scan(&o.UserID, &o.CanEdit, &o.DisplayName); err != nil {
			return nil, err
		}
		result = append(result, o)
	}
	return result, rows.Err()
}
