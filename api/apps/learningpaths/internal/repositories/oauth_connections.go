package repositories

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/oauth2"

	"tools.xdoubleu.com/internal/crypto"
	"tools.xdoubleu.com/internal/database/postgres"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

// OAuthConnectionsRepository stores one encrypted OAuth connection per
// (user, provider) in learningpaths.oauth_connections, separate from the
// provider-keyed global.oauth_connections.
type OAuthConnectionsRepository struct {
	db     postgres.DB
	sealer *crypto.Sealer
}

func NewOAuthConnectionsRepository(
	db postgres.DB, sealer *crypto.Sealer,
) *OAuthConnectionsRepository {
	return &OAuthConnectionsRepository{db: db, sealer: sealer}
}

type oauthConnectionRow struct {
	accessToken  []byte
	refreshToken []byte
	expiresAt    *time.Time
	connectedAt  time.Time
	updatedAt    time.Time
}

// Get returns the decrypted token and metadata for (userID, provider), or
// database.ErrResourceNotFound if not connected.
func (r *OAuthConnectionsRepository) Get(
	ctx context.Context, userID string, provider sharedmodels.OAuthProvider,
) (*oauth2.Token, *sharedmodels.OAuthConnection, error) {
	var row oauthConnectionRow
	err := r.db.QueryRow(ctx, `
		SELECT access_token, refresh_token, expires_at, connected_at, updated_at
		FROM learningpaths.oauth_connections
		WHERE user_id = $1 AND provider = $2
	`, userID, provider).Scan(
		&row.accessToken, &row.refreshToken, &row.expiresAt,
		&row.connectedAt, &row.updatedAt,
	)
	if err != nil {
		return nil, nil, postgres.PgxErrorToHTTPError(err)
	}

	tok, err := r.decryptToken(row)
	if err != nil {
		return nil, nil, err
	}

	return tok, rowToConnection(provider, userID, row), nil
}

// Upsert stores a token for (userID, provider), replacing any existing one.
// requestedScopes are recorded for oauthconn.ScopesAreStale.
func (r *OAuthConnectionsRepository) Upsert(
	ctx context.Context,
	userID string,
	provider sharedmodels.OAuthProvider,
	tok *oauth2.Token,
) error {
	access, refresh, err := r.encryptToken(tok)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx, `
		INSERT INTO learningpaths.oauth_connections
			(user_id, provider, access_token, refresh_token, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, provider) DO UPDATE SET
			access_token  = EXCLUDED.access_token,
			refresh_token = EXCLUDED.refresh_token,
			expires_at    = EXCLUDED.expires_at,
			connected_at  = now(),
			updated_at    = now()
	`, userID, provider, access, refresh, expiryPtr(tok))
	return err
}

// UpdateToken stores a refreshed token, preserving connected_at.
func (r *OAuthConnectionsRepository) UpdateToken(
	ctx context.Context,
	userID string,
	provider sharedmodels.OAuthProvider,
	tok *oauth2.Token,
) error {
	access, refresh, err := r.encryptToken(tok)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx, `
		UPDATE learningpaths.oauth_connections
		SET access_token = $3, refresh_token = $4, expires_at = $5,
		    updated_at = now()
		WHERE user_id = $1 AND provider = $2
	`, userID, provider, access, refresh, expiryPtr(tok))
	return err
}

// Delete removes userID's connection to provider, if any.
func (r *OAuthConnectionsRepository) Delete(
	ctx context.Context, userID string, provider sharedmodels.OAuthProvider,
) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM learningpaths.oauth_connections
		WHERE user_id = $1 AND provider = $2
	`, userID, provider)
	return err
}

// GetStatus returns connection metadata without the token, or
// database.ErrResourceNotFound if not connected.
func (r *OAuthConnectionsRepository) GetStatus(
	ctx context.Context, userID string, provider sharedmodels.OAuthProvider,
) (*sharedmodels.OAuthConnection, error) {
	var row oauthConnectionRow
	err := r.db.QueryRow(ctx, `
		SELECT connected_at, updated_at
		FROM learningpaths.oauth_connections
		WHERE user_id = $1 AND provider = $2
	`, userID, provider).Scan(&row.connectedAt, &row.updatedAt)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return rowToConnection(provider, userID, row), nil
}

// ForUser binds userID so the repository structurally satisfies oauthconn's
// provider-only connectionStore interface for that user.
func (r *OAuthConnectionsRepository) ForUser(userID string) UserScopedOAuthStore {
	return UserScopedOAuthStore{repo: r, userID: userID}
}

// UserScopedOAuthStore is OAuthConnectionsRepository bound to one userID.
type UserScopedOAuthStore struct {
	repo   *OAuthConnectionsRepository
	userID string
}

func (u UserScopedOAuthStore) Get(
	ctx context.Context, provider sharedmodels.OAuthProvider,
) (*oauth2.Token, *sharedmodels.OAuthConnection, error) {
	return u.repo.Get(ctx, u.userID, provider)
}

func (u UserScopedOAuthStore) UpdateToken(
	ctx context.Context, provider sharedmodels.OAuthProvider, tok *oauth2.Token,
) error {
	return u.repo.UpdateToken(ctx, u.userID, provider, tok)
}

func (r *OAuthConnectionsRepository) encryptToken(
	tok *oauth2.Token,
) ([]byte, []byte, error) {
	access, err := r.sealer.Encrypt([]byte(tok.AccessToken))
	if err != nil {
		return nil, nil, err
	}

	var refresh []byte
	if tok.RefreshToken != "" {
		refresh, err = r.sealer.Encrypt([]byte(tok.RefreshToken))
		if err != nil {
			return nil, nil, err
		}
	}
	return access, refresh, nil
}

func (r *OAuthConnectionsRepository) decryptToken(
	row oauthConnectionRow,
) (*oauth2.Token, error) {
	access, err := r.sealer.Decrypt(row.accessToken)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", sharedmodels.ErrDecryptFailed, err)
	}

	var refresh string
	if len(row.refreshToken) > 0 {
		refreshBytes, decErr := r.sealer.Decrypt(row.refreshToken)
		if decErr != nil {
			return nil, fmt.Errorf("%w: %w", sharedmodels.ErrDecryptFailed, decErr)
		}
		refresh = string(refreshBytes)
	}

	tok := &oauth2.Token{ //nolint:exhaustruct // token type/raw fields unused
		AccessToken:  string(access),
		RefreshToken: refresh,
	}
	if row.expiresAt != nil {
		tok.Expiry = *row.expiresAt
	}
	return tok, nil
}

func rowToConnection(
	provider sharedmodels.OAuthProvider, userID string, row oauthConnectionRow,
) *sharedmodels.OAuthConnection {
	//nolint:exhaustruct // GrantedScope/RequestedScope/Config not tracked here
	return &sharedmodels.OAuthConnection{
		Provider: provider,
		// Per-user table: the owner is always who connected it.
		ConnectedBy: userID,
		ConnectedAt: row.connectedAt,
		UpdatedAt:   row.updatedAt,
		ExpiresAt:   row.expiresAt,
	}
}

func expiryPtr(tok *oauth2.Token) *time.Time {
	if tok.Expiry.IsZero() {
		return nil
	}
	return &tok.Expiry
}
