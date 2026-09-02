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

// GetFamilyID returns the family the user currently belongs to. ok is false
// when the user has no membership row (an implicit family-of-one).
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

// EnsureFamily returns the family userID currently belongs to, creating a new
// solo family for them (and inserting their membership row) if they don't
// have one yet. This is the "implicit family-of-one" from a lazy-creation
// angle: callers that need a concrete family_id to key data by never have to
// special-case "no family yet".
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

	// Someone else may have raced us into creating userID's family between
	// the initial GetFamilyID and this transaction; re-read the winning row
	// rather than trusting the one we just (maybe redundantly) inserted.
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

// Invite creates or replaces a pending invite from fromUserID's family to
// toUserID. fromUserID's family is created first if they don't have one yet.
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

// AcceptInvite joins userID into the family they were invited to: deletes
// the invite and upserts their membership row. Returns the joined family_id.
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

// Leave removes userID's membership row. Per issue #1349's confirmed
// decision, data already merged into the family cannot be un-merged — the
// leaving user simply has no membership (and thus no family-scoped data)
// afterwards; the rest of the family keeps everything.
func (r *FamilyRepository) Leave(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM global.family_members WHERE user_id = $1`,
		userID,
	)
	return err
}
