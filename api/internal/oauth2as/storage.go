// Package oauth2as is a first-party embedded OAuth 2.1 authorization server
// (issue #1039), built on ory/fosite, backing the MCP server's OAuth flow —
// replacing Supabase as the authorization server.
package oauth2as

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/ory/fosite"
	"github.com/ory/fosite/handler/oauth2"
	"github.com/ory/fosite/handler/pkce"

	"tools.xdoubleu.com/internal/database/postgres"
)

// authCodeLifespan/accessTokenLifespan/etc bound how long a row is kept
// before it's considered expired; the actual token lifetimes are set on the
// fosite.Config passed to NewProvider (server.go) — these are only used to
// size the DB row's expires_at as a defensive upper bound for storage.Store's
// own bookkeeping (fosite itself, not this store, enforces token expiry).
const pkceRequestLifespan = 10 * time.Minute

// persistedRequest is the JSON shape stored in the oauth2_* tables' `request`
// JSONB column. fosite.Request's Client and Session fields are interfaces,
// so they can't be unmarshaled generically — this captures only the
// concrete data needed to reconstruct a fosite.Request, with Session stored
// as raw JSON unmarshaled into the caller-supplied session on read (the
// standard fosite storage pattern).
type persistedRequest struct {
	ID                string          `json:"id"`
	RequestedAt       time.Time       `json:"requested_at"`
	ClientID          string          `json:"client_id"`
	RequestedScope    []string        `json:"requested_scope"`
	GrantedScope      []string        `json:"granted_scope"`
	Form              url.Values      `json:"form"`
	RequestedAudience []string        `json:"requested_audience"`
	GrantedAudience   []string        `json:"granted_audience"`
	Session           json.RawMessage `json:"session"`
}

// Store implements fosite's oauth2.CoreStorage, oauth2.TokenRevocationStorage,
// pkce.PKCERequestStorage, and fosite.Storage (ClientManager) against the
// auth.oauth2_* tables.
type Store struct {
	db postgres.DB
}

func NewStore(db postgres.DB) *Store {
	return &Store{db: db}
}

var (
	_ fosite.Storage                = (*Store)(nil)
	_ oauth2.CoreStorage            = (*Store)(nil)
	_ oauth2.TokenRevocationStorage = (*Store)(nil)
	_ pkce.PKCERequestStorage       = (*Store)(nil)
)

func toPersisted(requester fosite.Requester) (*persistedRequest, error) {
	sessionJSON, err := json.Marshal(requester.GetSession())
	if err != nil {
		return nil, err
	}
	return &persistedRequest{
		ID:                requester.GetID(),
		RequestedAt:       requester.GetRequestedAt(),
		ClientID:          requester.GetClient().GetID(),
		RequestedScope:    []string(requester.GetRequestedScopes()),
		GrantedScope:      []string(requester.GetGrantedScopes()),
		Form:              requester.GetRequestForm(),
		RequestedAudience: []string(requester.GetRequestedAudience()),
		GrantedAudience:   []string(requester.GetGrantedAudience()),
		Session:           sessionJSON,
	}, nil
}

func (s *Store) fromPersisted(
	ctx context.Context,
	pr *persistedRequest,
	session fosite.Session,
) (fosite.Requester, error) {
	if session != nil && len(pr.Session) > 0 {
		if err := json.Unmarshal(pr.Session, session); err != nil {
			return nil, err
		}
	}

	client, err := s.GetClient(ctx, pr.ClientID)
	if err != nil {
		return nil, err
	}

	//nolint:exhaustruct //Lang is unused by this server
	return &fosite.Request{
		ID:                pr.ID,
		RequestedAt:       pr.RequestedAt,
		Client:            client,
		RequestedScope:    pr.RequestedScope,
		GrantedScope:      pr.GrantedScope,
		Form:              pr.Form,
		Session:           session,
		RequestedAudience: pr.RequestedAudience,
		GrantedAudience:   pr.GrantedAudience,
	}, nil
}

func subjectOf(requester fosite.Requester) *string {
	if requester.GetSession() == nil {
		return nil
	}
	sub := requester.GetSession().GetSubject()
	if sub == "" {
		return nil
	}
	return &sub
}

func (s *Store) createRequest(
	ctx context.Context,
	table, signature string,
	requester fosite.Requester,
	expiresAt time.Time,
) error {
	pr, err := toPersisted(requester)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(pr)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(ctx, `
		INSERT INTO auth.`+table+`
			(signature, request, client_id, user_id, active, expires_at)
		VALUES ($1, $2, $3, $4, true, $5)
		ON CONFLICT (signature) DO UPDATE SET
			request = EXCLUDED.request, active = true, expires_at = EXCLUDED.expires_at
	`, signature, raw, pr.ClientID, subjectOf(requester), expiresAt)
	return err
}

func (s *Store) getRequest(
	ctx context.Context,
	table, signature string,
	session fosite.Session,
) (fosite.Requester, error) {
	var raw []byte
	var active bool

	err := s.db.QueryRow(ctx, `
		SELECT request, active FROM auth.`+table+` WHERE signature = $1
	`, signature).Scan(&raw, &active)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fosite.ErrNotFound
		}
		return nil, err
	}

	var pr persistedRequest
	if err = json.Unmarshal(raw, &pr); err != nil {
		return nil, err
	}
	requester, err := s.fromPersisted(ctx, &pr, session)
	if err != nil {
		return nil, err
	}
	if !active && table == "oauth2_auth_codes" {
		return requester, fosite.ErrInvalidatedAuthorizeCode
	}
	return requester, nil
}

func (s *Store) deleteRequest(ctx context.Context, table, signature string) error {
	_, err := s.db.Exec(
		ctx, `DELETE FROM auth.`+table+` WHERE signature = $1`, signature,
	)
	return err
}

// --- AuthorizeCodeStorage ---

func (s *Store) CreateAuthorizeCodeSession(
	ctx context.Context, code string, request fosite.Requester,
) error {
	return s.createRequest(
		ctx, "oauth2_auth_codes", code, request,
		time.Now().Add(pkceRequestLifespan),
	)
}

func (s *Store) GetAuthorizeCodeSession(
	ctx context.Context, code string, session fosite.Session,
) (fosite.Requester, error) {
	return s.getRequest(ctx, "oauth2_auth_codes", code, session)
}

func (s *Store) InvalidateAuthorizeCodeSession(ctx context.Context, code string) error {
	_, err := s.db.Exec(
		ctx,
		`UPDATE auth.oauth2_auth_codes SET active = false WHERE signature = $1`,
		code,
	)
	return err
}

// --- AccessTokenStorage ---

func (s *Store) CreateAccessTokenSession(
	ctx context.Context, signature string, request fosite.Requester,
) error {
	return s.createRequest(
		ctx, "oauth2_access_tokens", signature, request,
		request.GetSession().GetExpiresAt(fosite.AccessToken),
	)
}

func (s *Store) GetAccessTokenSession(
	ctx context.Context, signature string, session fosite.Session,
) (fosite.Requester, error) {
	return s.getRequest(ctx, "oauth2_access_tokens", signature, session)
}

func (s *Store) DeleteAccessTokenSession(ctx context.Context, signature string) error {
	return s.deleteRequest(ctx, "oauth2_access_tokens", signature)
}

// --- RefreshTokenStorage ---

func (s *Store) CreateRefreshTokenSession(
	ctx context.Context, signature, _ string, request fosite.Requester,
) error {
	return s.createRequest(
		ctx, "oauth2_refresh_tokens", signature, request,
		request.GetSession().GetExpiresAt(fosite.RefreshToken),
	)
}

func (s *Store) GetRefreshTokenSession(
	ctx context.Context, signature string, session fosite.Session,
) (fosite.Requester, error) {
	return s.getRequest(ctx, "oauth2_refresh_tokens", signature, session)
}

func (s *Store) DeleteRefreshTokenSession(ctx context.Context, signature string) error {
	return s.deleteRequest(ctx, "oauth2_refresh_tokens", signature)
}

func (s *Store) RotateRefreshToken(
	ctx context.Context, requestID string, refreshTokenSignature string,
) error {
	_, err := s.db.Exec(ctx, `
		UPDATE auth.oauth2_refresh_tokens SET active = false
		WHERE signature = $1 AND request->>'id' = $2
	`, refreshTokenSignature, requestID)
	return err
}

// --- TokenRevocationStorage ---

func (s *Store) RevokeRefreshToken(ctx context.Context, requestID string) error {
	_, err := s.db.Exec(ctx, `
		DELETE FROM auth.oauth2_refresh_tokens WHERE request->>'id' = $1
	`, requestID)
	return err
}

func (s *Store) RevokeAccessToken(ctx context.Context, requestID string) error {
	_, err := s.db.Exec(ctx, `
		DELETE FROM auth.oauth2_access_tokens WHERE request->>'id' = $1
	`, requestID)
	return err
}

// --- PKCERequestStorage ---

func (s *Store) CreatePKCERequestSession(
	ctx context.Context, signature string, requester fosite.Requester,
) error {
	pr, err := toPersisted(requester)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(pr)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(ctx, `
		INSERT INTO auth.oauth2_pkce_requests (signature, request, expires_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (signature) DO UPDATE SET request = EXCLUDED.request
	`, signature, raw, time.Now().Add(pkceRequestLifespan))
	return err
}

func (s *Store) GetPKCERequestSession(
	ctx context.Context, signature string, session fosite.Session,
) (fosite.Requester, error) {
	var raw []byte
	err := s.db.QueryRow(ctx, `
		SELECT request FROM auth.oauth2_pkce_requests WHERE signature = $1
	`, signature).Scan(&raw)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fosite.ErrNotFound
		}
		return nil, err
	}
	var pr persistedRequest
	if err = json.Unmarshal(raw, &pr); err != nil {
		return nil, err
	}
	return s.fromPersisted(ctx, &pr, session)
}

func (s *Store) DeletePKCERequestSession(ctx context.Context, signature string) error {
	_, err := s.db.Exec(
		ctx, `DELETE FROM auth.oauth2_pkce_requests WHERE signature = $1`, signature,
	)
	return err
}

// --- ClientManager ---

func (s *Store) GetClient(ctx context.Context, id string) (fosite.Client, error) {
	var (
		c            fosite.DefaultClient
		secretHash   *string
		redirectURIs []string
		grantTypes   []string
		respTypes    []string
		scopes       []string
	)
	err := s.db.QueryRow(ctx, `
		SELECT id, secret_hash, redirect_uris, grant_types, response_types, scopes, public
		FROM auth.oauth2_clients WHERE id = $1
	`, id).Scan(
		&c.ID, &secretHash, &redirectURIs, &grantTypes, &respTypes, &scopes, &c.Public,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fosite.ErrNotFound
		}
		return nil, err
	}
	if secretHash != nil {
		c.Secret = []byte(*secretHash)
	}
	c.RedirectURIs = redirectURIs
	c.GrantTypes = grantTypes
	c.ResponseTypes = respTypes
	c.Scopes = scopes
	return &c, nil
}

// GetClientName returns the human-readable client_name recorded at
// registration time, for the web consent page (client_name isn't part of
// fosite.Client, which GetClient returns).
func (s *Store) GetClientName(ctx context.Context, id string) (string, error) {
	var name string
	err := s.db.QueryRow(
		ctx, `SELECT client_name FROM auth.oauth2_clients WHERE id = $1`, id,
	).Scan(&name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", fosite.ErrNotFound
		}
		return "", err
	}
	return name, nil
}

// ClientAssertionJWTValid/SetClientAssertionJWT back the private_key_jwt
// client-authentication method, which this public-client-only, non-JWT-auth
// authorization server never uses — stubbed out to satisfy ClientManager.
func (s *Store) ClientAssertionJWTValid(_ context.Context, _ string) error {
	return nil
}

func (s *Store) SetClientAssertionJWT(_ context.Context, _ string, _ time.Time) error {
	return nil
}
