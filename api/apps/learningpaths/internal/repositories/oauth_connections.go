package repositories

import (
	"context"
	"time"

	"golang.org/x/oauth2"

	"tools.xdoubleu.com/internal/crypto"
	"tools.xdoubleu.com/internal/database/postgres"
	sharedmodels "tools.xdoubleu.com/internal/models"
	sharedrepos "tools.xdoubleu.com/internal/repositories"
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
	accessToken    []byte
	refreshToken   []byte
	expiresAt      *time.Time
	connectedAt    time.Time
	updatedAt      time.Time
	requestedScope *string
}

// Get returns the decrypted token and metadata for (userID, provider), or
// database.ErrResourceNotFound if not connected.
func (r *OAuthConnectionsRepository) Get(
	ctx context.Context, userID string, provider sharedmodels.OAuthProvider,
) (*oauth2.Token, *sharedmodels.OAuthConnection, error) {
	var row oauthConnectionRow
	err := r.db.QueryRow(ctx, `
		SELECT access_token, refresh_token, expires_at, connected_at, updated_at,
			requested_scope
		FROM learningpaths.oauth_connections
		WHERE user_id = $1 AND provider = $2
	`, userID, provider).Scan(
		&row.accessToken, &row.refreshToken, &row.expiresAt,
		&row.connectedAt, &row.updatedAt, &row.requestedScope,
	)
	if err != nil {
		return nil, nil, postgres.PgxErrorToHTTPError(err)
	}

	tok, err := sharedrepos.UnsealToken(
		r.sealer, row.accessToken, row.refreshToken, row.expiresAt,
	)
	if err != nil {
		return nil, nil, err
	}

	return tok, rowToConnection(provider, userID, row), nil
}

// Upsert stores a token for (userID, provider), replacing any existing one.
// requestedScope is recorded for oauthconn.ScopesAreStale.
func (r *OAuthConnectionsRepository) Upsert(
	ctx context.Context,
	userID string,
	provider sharedmodels.OAuthProvider,
	tok *oauth2.Token,
	requestedScope string,
) error {
	access, refresh, err := sharedrepos.SealToken(r.sealer, tok)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx, `
		INSERT INTO learningpaths.oauth_connections
			(user_id, provider, access_token, refresh_token, expires_at,
			 requested_scope)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, provider) DO UPDATE SET
			access_token    = EXCLUDED.access_token,
			refresh_token   = EXCLUDED.refresh_token,
			expires_at      = EXCLUDED.expires_at,
			requested_scope = EXCLUDED.requested_scope,
			connected_at    = now(),
			updated_at      = now()
	`, userID, provider, access, refresh, expiryPtr(tok), requestedScope)
	return err
}

// UpdateToken stores a refreshed token, preserving connected_at.
func (r *OAuthConnectionsRepository) UpdateToken(
	ctx context.Context,
	userID string,
	provider sharedmodels.OAuthProvider,
	tok *oauth2.Token,
) error {
	access, refresh, err := sharedrepos.SealToken(r.sealer, tok)
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
		SELECT connected_at, updated_at, requested_scope
		FROM learningpaths.oauth_connections
		WHERE user_id = $1 AND provider = $2
	`, userID, provider).Scan(&row.connectedAt, &row.updatedAt, &row.requestedScope)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return rowToConnection(provider, userID, row), nil
}

// Status is GetStatus for the user's Todoist connection; it satisfies the
// todoistConnections interface the service depends on.
func (r *OAuthConnectionsRepository) Status(
	ctx context.Context, userID string,
) (*sharedmodels.OAuthConnection, error) {
	return r.GetStatus(ctx, userID, sharedmodels.OAuthProviderTodoist)
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

func rowToConnection(
	provider sharedmodels.OAuthProvider, userID string, row oauthConnectionRow,
) *sharedmodels.OAuthConnection {
	//nolint:exhaustruct // GrantedScope/Config not tracked here
	conn := &sharedmodels.OAuthConnection{
		Provider: provider,
		// Per-user table: the owner is always who connected it.
		ConnectedBy: userID,
		ConnectedAt: row.connectedAt,
		UpdatedAt:   row.updatedAt,
		ExpiresAt:   row.expiresAt,
	}
	if row.requestedScope != nil {
		conn.RequestedScope = *row.requestedScope
	}
	return conn
}

func expiryPtr(tok *oauth2.Token) *time.Time {
	if tok.Expiry.IsZero() {
		return nil
	}
	return &tok.Expiry
}
