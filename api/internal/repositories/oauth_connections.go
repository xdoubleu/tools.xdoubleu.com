package repositories

import (
	"context"
	"errors"
	"time"

	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"
	"golang.org/x/oauth2"

	"tools.xdoubleu.com/internal/crypto"
	"tools.xdoubleu.com/internal/models"
)

// ErrEncryptionNotConfigured is returned when OAUTH_TOKEN_ENC_KEY isn't set,
// so no OAuth connection can be stored or read.
var ErrEncryptionNotConfigured = errors.New(
	"repositories: OAUTH_TOKEN_ENC_KEY not configured",
)

// OAuthConnectionsRepository stores one OAuth connection per external
// provider (global.oauth_connections). Access/refresh tokens are encrypted
// at rest via sealer; every other repository/service only ever sees a live
// *oauth2.Token, never the stored bytes.
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
	connectedBy  string
	connectedAt  time.Time
	updatedAt    time.Time
}

// Get returns the decrypted token plus connection metadata for provider, or
// database.ErrResourceNotFound if it isn't connected.
func (r *OAuthConnectionsRepository) Get(
	ctx context.Context, provider models.OAuthProvider,
) (*oauth2.Token, *models.OAuthConnection, error) {
	var row oauthConnectionRow
	err := r.db.QueryRow(ctx, `
		SELECT access_token, refresh_token, expires_at, connected_by,
		       connected_at, updated_at
		FROM global.oauth_connections
		WHERE provider = $1
	`, provider).Scan(
		&row.accessToken, &row.refreshToken, &row.expiresAt,
		&row.connectedBy, &row.connectedAt, &row.updatedAt,
	)
	if err != nil {
		return nil, nil, postgres.PgxErrorToHTTPError(err)
	}

	tok, err := r.decryptToken(row)
	if err != nil {
		return nil, nil, err
	}

	return tok, rowToConnection(provider, row), nil
}

// Upsert stores a fresh token for provider, replacing any existing connection
// and recording connectedBy as the admin who authorized it.
func (r *OAuthConnectionsRepository) Upsert(
	ctx context.Context,
	provider models.OAuthProvider,
	tok *oauth2.Token,
	connectedBy string,
) error {
	access, refresh, err := r.encryptToken(tok)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx, `
		INSERT INTO global.oauth_connections
			(provider, access_token, refresh_token, expires_at, connected_by)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (provider) DO UPDATE SET
			access_token  = EXCLUDED.access_token,
			refresh_token = EXCLUDED.refresh_token,
			expires_at    = EXCLUDED.expires_at,
			connected_by  = EXCLUDED.connected_by,
			connected_at  = now(),
			updated_at    = now()
	`, provider, access, refresh, expiryPtr(tok), connectedBy)
	return err
}

// UpdateToken re-encrypts and stores a rotated token in place, preserving the
// existing connected_by/connected_at. Called after a transparent refresh.
func (r *OAuthConnectionsRepository) UpdateToken(
	ctx context.Context, provider models.OAuthProvider, tok *oauth2.Token,
) error {
	access, refresh, err := r.encryptToken(tok)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx, `
		UPDATE global.oauth_connections
		SET access_token = $2, refresh_token = $3, expires_at = $4,
		    updated_at = now()
		WHERE provider = $1
	`, provider, access, refresh, expiryPtr(tok))
	return err
}

func (r *OAuthConnectionsRepository) Delete(
	ctx context.Context, provider models.OAuthProvider,
) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM global.oauth_connections WHERE provider = $1
	`, provider)
	return err
}

// List returns every connected provider's status (no tokens), for the admin
// UI. Providers with no row simply aren't in the result.
func (r *OAuthConnectionsRepository) List(
	ctx context.Context,
) ([]models.OAuthConnection, error) {
	rows, err := r.db.Query(ctx, `
		SELECT provider, expires_at, connected_by, connected_at, updated_at
		FROM global.oauth_connections
		ORDER BY provider
	`)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	var connections []models.OAuthConnection
	for rows.Next() {
		var (
			provider models.OAuthProvider
			row      oauthConnectionRow
		)
		if scanErr := rows.Scan(
			&provider, &row.expiresAt, &row.connectedBy,
			&row.connectedAt, &row.updatedAt,
		); scanErr != nil {
			return nil, scanErr
		}
		connections = append(connections, *rowToConnection(provider, row))
	}
	return connections, rows.Err()
}

func (r *OAuthConnectionsRepository) encryptToken(
	tok *oauth2.Token,
) ([]byte, []byte, error) {
	if r.sealer == nil {
		return nil, nil, ErrEncryptionNotConfigured
	}

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
	if r.sealer == nil {
		return nil, ErrEncryptionNotConfigured
	}

	access, err := r.sealer.Decrypt(row.accessToken)
	if err != nil {
		return nil, err
	}

	var refresh string
	if len(row.refreshToken) > 0 {
		refreshBytes, decErr := r.sealer.Decrypt(row.refreshToken)
		if decErr != nil {
			return nil, decErr
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
	provider models.OAuthProvider, row oauthConnectionRow,
) *models.OAuthConnection {
	return &models.OAuthConnection{
		Provider:    provider,
		ConnectedBy: row.connectedBy,
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
