package repositories

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/models"
)

type FamilyRepository struct {
	db postgres.DB
}

func NewFamilyRepository(db postgres.DB) *FamilyRepository {
	return &FamilyRepository{db: db}
}

// GetFamilyID returns the user's family; ok is false for a family-of-one.
func (r *FamilyRepository) GetFamilyID(
	ctx context.Context,
	userID string,
) (uuid.UUID, bool, error) {
	var familyID uuid.UUID
	err := r.db.QueryRow(ctx, `
		SELECT family_id FROM global.family_members WHERE user_id = $1`,
		userID,
	).Scan(&familyID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, err
	}
	return familyID, true, nil
}

// ListMembers returns the user IDs belonging to familyID.
func (r *FamilyRepository) ListMembers(
	ctx context.Context,
	familyID uuid.UUID,
) ([]string, error) {
	rows, err := r.db.Query(ctx, `
		SELECT user_id FROM global.family_members
		WHERE family_id = $1
		ORDER BY joined_at`,
		familyID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []string
	for rows.Next() {
		var userID string
		if err = rows.Scan(&userID); err != nil {
			return nil, err
		}
		result = append(result, userID)
	}
	return result, rows.Err()
}

// MemberDisplayNames maps each member to their display name ("" if unset).
func (r *FamilyRepository) MemberDisplayNames(
	ctx context.Context,
	familyID uuid.UUID,
) (map[string]string, error) {
	rows, err := r.db.Query(ctx, `
		SELECT user_id, display_name FROM global.family_members
		WHERE family_id = $1`,
		familyID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	names := map[string]string{}
	for rows.Next() {
		var userID, displayName string
		if err = rows.Scan(&userID, &displayName); err != nil {
			return nil, err
		}
		names[userID] = displayName
	}
	return names, rows.Err()
}

// SetDisplayName sets userID's display name; a no-op without a membership row.
func (r *FamilyRepository) SetDisplayName(
	ctx context.Context,
	userID, displayName string,
) error {
	_, err := r.db.Exec(ctx, `
		UPDATE global.family_members SET display_name = $2 WHERE user_id = $1`,
		userID, displayName,
	)
	return err
}

// EnsureFamily returns userID's family, creating a solo one if needed, so
// callers always have a family_id.
func (r *FamilyRepository) EnsureFamily(
	ctx context.Context,
	userID string,
) (uuid.UUID, error) {
	familyID, ok, err := r.GetFamilyID(ctx, userID)
	if err != nil {
		return uuid.Nil, err
	}
	if ok {
		return familyID, nil
	}

	//nolint:exhaustruct // default tx options
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return uuid.Nil, postgres.PgxErrorToHTTPError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err = tx.QueryRow(ctx, `
		INSERT INTO global.families DEFAULT VALUES RETURNING id`,
	).Scan(&familyID); err != nil {
		return uuid.Nil, err
	}

	if _, err = tx.Exec(ctx, `
		INSERT INTO global.family_members (user_id, family_id)
		VALUES ($1, $2)
		ON CONFLICT (user_id) DO NOTHING`,
		userID, familyID,
	); err != nil {
		return uuid.Nil, err
	}

	// Re-read in case a concurrent call created the family first.
	if err = tx.QueryRow(ctx, `
		SELECT family_id FROM global.family_members WHERE user_id = $1`,
		userID,
	).Scan(&familyID); err != nil {
		return uuid.Nil, err
	}

	if err = tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return familyID, nil
}

// Invite creates or replaces an invite from fromUserID's family (created if
// needed) to toUserID.
func (r *FamilyRepository) Invite(
	ctx context.Context,
	fromUserID, toUserID string,
) error {
	familyID, err := r.EnsureFamily(ctx, fromUserID)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx, `
		INSERT INTO global.family_invites (family_id, from_user_id, to_user_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (to_user_id)
		DO UPDATE SET family_id = EXCLUDED.family_id,
		              from_user_id = EXCLUDED.from_user_id,
		              created_at = now()`,
		familyID, fromUserID, toUserID,
	)
	return err
}

// GetInvite returns the pending invite addressed to userID, if any.
func (r *FamilyRepository) GetInvite(
	ctx context.Context,
	userID string,
) (models.FamilyInvite, bool, error) {
	var inv models.FamilyInvite
	err := r.db.QueryRow(ctx, `
		SELECT id, family_id, from_user_id, to_user_id, created_at
		FROM global.family_invites
		WHERE to_user_id = $1`,
		userID,
	).Scan(&inv.ID, &inv.FamilyID, &inv.FromUserID, &inv.ToUserID, &inv.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.FamilyInvite{}, false, nil //nolint:exhaustruct // zero value
	}
	if err != nil {
		return models.FamilyInvite{}, false, err
	}
	return inv, true, nil
}

// DeclineInvite deletes the pending invite addressed to userID.
func (r *FamilyRepository) DeclineInvite(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM global.family_invites WHERE to_user_id = $1`,
		userID,
	)
	return err
}

// AcceptInvite deletes the invite, upserts the membership and returns the
// family_id.
func (r *FamilyRepository) AcceptInvite(
	ctx context.Context,
	userID string,
) (uuid.UUID, error) {
	//nolint:exhaustruct // default tx options
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return uuid.Nil, postgres.PgxErrorToHTTPError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var familyID uuid.UUID
	err = tx.QueryRow(ctx, `
		DELETE FROM global.family_invites
		WHERE to_user_id = $1
		RETURNING family_id`,
		userID,
	).Scan(&familyID)
	if err != nil {
		return uuid.Nil, err
	}

	if _, err = tx.Exec(ctx, `
		INSERT INTO global.family_members (user_id, family_id)
		VALUES ($1, $2)
		ON CONFLICT (user_id)
		DO UPDATE SET family_id = EXCLUDED.family_id, joined_at = now()`,
		userID, familyID,
	); err != nil {
		return uuid.Nil, err
	}

	if err = tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return familyID, nil
}

// Leave removes userID's membership; family data stays with the family.
func (r *FamilyRepository) Leave(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM global.family_members WHERE user_id = $1`,
		userID,
	)
	return err
}
