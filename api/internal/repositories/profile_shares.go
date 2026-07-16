package repositories

import (
	"context"

	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"

	"tools.xdoubleu.com/internal/models"
)

// ProfileSharesRepository stores the opaque tokens behind public profile
// links (global.profile_shares). A token resolves to the owning user for
// the unauthenticated profile RPCs in the books and games apps.
type ProfileSharesRepository struct {
	db postgres.DB
}

func NewProfileSharesRepository(db postgres.DB) *ProfileSharesRepository {
	return &ProfileSharesRepository{db: db}
}

// Get returns the user's share, or database.ErrResourceNotFound when none
// exists.
func (r *ProfileSharesRepository) Get(
	ctx context.Context,
	userID string,
) (*models.ProfileShare, error) {
	var share models.ProfileShare
	err := r.db.QueryRow(ctx, `
		SELECT user_id, token, created_at
		FROM global.profile_shares
		WHERE user_id = $1
	`, userID).Scan(&share.UserID, &share.Token, &share.CreatedAt)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return &share, nil
}

// Upsert replaces the user's share token, invalidating any previous link.
func (r *ProfileSharesRepository) Upsert(
	ctx context.Context,
	userID, token string,
) (*models.ProfileShare, error) {
	var share models.ProfileShare
	err := r.db.QueryRow(ctx, `
		INSERT INTO global.profile_shares (user_id, token)
		VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE SET
			token      = EXCLUDED.token,
			created_at = now()
		RETURNING user_id, token, created_at
	`, userID, token).Scan(&share.UserID, &share.Token, &share.CreatedAt)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return &share, nil
}

func (r *ProfileSharesRepository) Delete(
	ctx context.Context,
	userID string,
) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM global.profile_shares WHERE user_id = $1
	`, userID)
	return err
}

// GetUserIDByToken resolves a share token to its owner. Returns
// database.ErrResourceNotFound when the token is unknown.
func (r *ProfileSharesRepository) GetUserIDByToken(
	ctx context.Context,
	token string,
) (string, error) {
	var userID string
	err := r.db.QueryRow(ctx, `
		SELECT user_id FROM global.profile_shares WHERE token = $1
	`, token).Scan(&userID)
	if err != nil {
		return "", postgres.PgxErrorToHTTPError(err)
	}
	return userID, nil
}
