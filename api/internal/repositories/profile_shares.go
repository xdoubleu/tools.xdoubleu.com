package repositories

import (
	"context"

	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/models"
)

// ProfileSharesRepository stores public dashboard share tokens, one per
// (user, kind).
type ProfileSharesRepository struct {
	db postgres.DB
}

func NewProfileSharesRepository(db postgres.DB) *ProfileSharesRepository {
	return &ProfileSharesRepository{db: db}
}

// Get returns the user's share, or database.ErrResourceNotFound.
func (r *ProfileSharesRepository) Get(
	ctx context.Context,
	userID string,
	app models.DashboardKind,
) (*models.ProfileShare, error) {
	var share models.ProfileShare
	err := r.db.QueryRow(ctx, `
		SELECT user_id, app, token, created_at
		FROM global.profile_shares
		WHERE user_id = $1 AND app = $2
	`, userID, app).Scan(&share.UserID, &share.App, &share.Token, &share.CreatedAt)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return &share, nil
}

// Upsert replaces the share token, invalidating the old link.
func (r *ProfileSharesRepository) Upsert(
	ctx context.Context,
	userID string,
	app models.DashboardKind,
	token string,
) (*models.ProfileShare, error) {
	var share models.ProfileShare
	err := r.db.QueryRow(ctx, `
		INSERT INTO global.profile_shares (user_id, app, token)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, app) DO UPDATE SET
			token      = EXCLUDED.token,
			created_at = now()
		RETURNING user_id, app, token, created_at
	`, userID, app, token).Scan(&share.UserID, &share.App, &share.Token, &share.CreatedAt)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return &share, nil
}

func (r *ProfileSharesRepository) Delete(
	ctx context.Context,
	userID string,
	app models.DashboardKind,
) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM global.profile_shares WHERE user_id = $1 AND app = $2
	`, userID, app)
	return err
}

// ResolveToken resolves a token for kind to its owner's ID and display name;
// database.ErrResourceNotFound if unknown or for another kind.
func (r *ProfileSharesRepository) ResolveToken(
	ctx context.Context,
	token string,
	app models.DashboardKind,
) (string, string, error) {
	var userID, displayName string
	err := r.db.QueryRow(ctx, `
		SELECT s.user_id, COALESCE(u.display_name, '')
		FROM global.profile_shares s
		LEFT JOIN global.app_users u ON u.id = s.user_id
		WHERE s.token = $1 AND s.app = $2
	`, token, app).Scan(&userID, &displayName)
	if err != nil {
		return "", "", postgres.PgxErrorToHTTPError(err)
	}
	return userID, displayName, nil
}
