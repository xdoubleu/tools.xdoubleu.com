package oauth2as_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ory/fosite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/oauth2as"
	"tools.xdoubleu.com/internal/testhelper"
)

func newTestStore(t *testing.T) (*oauth2as.Store, *pgxpool.Pool) {
	t.Helper()
	db := testhelper.ConnectTestDB(testhelper.NewTestConfig().DBDsn)
	t.Cleanup(db.Close)
	return oauth2as.NewStore(db), db
}

func TestRegisterClient_Validation(t *testing.T) {
	_, db := newTestStore(t)
	ctx := context.Background()

	//nolint:exhaustruct //ClientName is optional
	_, err := oauth2as.RegisterClient(ctx, db, oauth2as.ClientMetadata{
		RedirectURIs: nil,
	})
	require.Error(t, err, "empty redirect_uris must be rejected")

	//nolint:exhaustruct //ClientName is optional
	_, err = oauth2as.RegisterClient(ctx, db, oauth2as.ClientMetadata{
		RedirectURIs: []string{"http://evil.example.com/callback"},
	})
	require.Error(t, err, "non-HTTPS non-localhost redirect_uris must be rejected")

	client, err := oauth2as.RegisterClient(ctx, db, oauth2as.ClientMetadata{
		RedirectURIs: []string{"http://localhost:8080/callback"},
		ClientName:   "test client",
	})
	require.NoError(t, err, "localhost redirect_uris are allowed for dev")
	assert.NotEmpty(t, client.ID)
	assert.True(t, client.Public)
	assert.Nil(t, client.Secret)

	//nolint:exhaustruct //ClientName is optional
	client2, err := oauth2as.RegisterClient(ctx, db, oauth2as.ClientMetadata{
		RedirectURIs: []string{"https://example.com/callback"},
	})
	require.NoError(t, err)
	assert.NotEmpty(t, client2.ID)
}

// TestRegisterClient_MalformedRedirectURI covers validateRedirectURI's
// url.Parse error branch specifically, distinct from a validly-parsed but
// disallowed-scheme URI.
func TestRegisterClient_MalformedRedirectURI(t *testing.T) {
	_, db := newTestStore(t)
	ctx := context.Background()

	//nolint:exhaustruct //ClientName is optional
	_, err := oauth2as.RegisterClient(ctx, db, oauth2as.ClientMetadata{
		RedirectURIs: []string{"http://[::1"},
	})
	require.Error(t, err, "an unparsable redirect_uri must be rejected")
}

func TestStore_GetClient_RoundTrip(t *testing.T) {
	store, db := newTestStore(t)
	ctx := context.Background()

	client, err := oauth2as.RegisterClient(ctx, db, oauth2as.ClientMetadata{
		RedirectURIs: []string{"https://example.com/callback"},
		ClientName:   "roundtrip client",
	})
	require.NoError(t, err)

	got, err := store.GetClient(ctx, client.ID)
	require.NoError(t, err)
	assert.Equal(t, client.ID, got.GetID())
	assert.Equal(t, []string{"https://example.com/callback"}, got.GetRedirectURIs())
	assert.True(t, got.IsPublic())

	name, err := store.GetClientName(ctx, client.ID)
	require.NoError(t, err)
	assert.Equal(t, "roundtrip client", name)

	_, err = store.GetClient(ctx, "does-not-exist")
	require.ErrorIs(t, err, fosite.ErrNotFound)
}

// fakeSession is a minimal fosite.Session for storage round-trip tests.
type fakeSession struct {
	Subject   string
	ExpiresAt map[fosite.TokenType]time.Time
}

func (s *fakeSession) SetExpiresAt(key fosite.TokenType, exp time.Time) {
	if s.ExpiresAt == nil {
		s.ExpiresAt = make(map[fosite.TokenType]time.Time)
	}
	s.ExpiresAt[key] = exp
}

func (s *fakeSession) GetExpiresAt(key fosite.TokenType) time.Time {
	if s.ExpiresAt == nil {
		return time.Time{}
	}
	return s.ExpiresAt[key]
}
func (s *fakeSession) GetUsername() string { return "" }
func (s *fakeSession) GetSubject() string  { return s.Subject }
func (s *fakeSession) Clone() fosite.Session {
	clone := *s
	return &clone
}

func TestStore_AccessTokenSession_RoundTrip(t *testing.T) {
	store, db := newTestStore(t)
	ctx := context.Background()

	//nolint:exhaustruct //ClientName is optional
	client, err := oauth2as.RegisterClient(ctx, db, oauth2as.ClientMetadata{
		RedirectURIs: []string{"https://example.com/callback"},
	})
	require.NoError(t, err)

	//nolint:exhaustruct //ExpiresAt is set below via SetExpiresAt
	session := &fakeSession{Subject: uuid.NewString()}
	session.SetExpiresAt(fosite.AccessToken, time.Now().Add(time.Hour))

	//nolint:exhaustruct //other fosite.Request fields are optional for this test
	request := &fosite.Request{
		ID:          uuid.NewString(),
		RequestedAt: time.Now(),
		Client:      client,
		Session:     session,
	}

	signature := uuid.NewString()
	require.NoError(t, store.CreateAccessTokenSession(ctx, signature, request))

	//nolint:exhaustruct //populated by GetAccessTokenSession's json.Unmarshal
	got, err := store.GetAccessTokenSession(ctx, signature, &fakeSession{})
	require.NoError(t, err)
	assert.Equal(t, session.Subject, got.GetSession().GetSubject())
	assert.Equal(t, client.ID, got.GetClient().GetID())

	require.NoError(t, store.DeleteAccessTokenSession(ctx, signature))
	//nolint:exhaustruct //populated by GetAccessTokenSession's json.Unmarshal
	_, err = store.GetAccessTokenSession(ctx, signature, &fakeSession{})
	require.ErrorIs(t, err, fosite.ErrNotFound)
}

func TestStore_AuthorizeCodeSession_Invalidate(t *testing.T) {
	store, db := newTestStore(t)
	ctx := context.Background()

	//nolint:exhaustruct //ClientName is optional
	client, err := oauth2as.RegisterClient(ctx, db, oauth2as.ClientMetadata{
		RedirectURIs: []string{"https://example.com/callback"},
	})
	require.NoError(t, err)

	//nolint:exhaustruct //ExpiresAt is unused by this test
	session := &fakeSession{Subject: uuid.NewString()}
	//nolint:exhaustruct //other fosite.Request fields are optional for this test
	request := &fosite.Request{
		ID:          uuid.NewString(),
		RequestedAt: time.Now(),
		Client:      client,
		Session:     session,
	}

	code := uuid.NewString()
	require.NoError(t, store.CreateAuthorizeCodeSession(ctx, code, request))

	//nolint:exhaustruct //populated by GetAuthorizeCodeSession's json.Unmarshal
	_, err = store.GetAuthorizeCodeSession(ctx, code, &fakeSession{})
	require.NoError(t, err)

	require.NoError(t, store.InvalidateAuthorizeCodeSession(ctx, code))
	//nolint:exhaustruct //populated by GetAuthorizeCodeSession's json.Unmarshal
	_, err = store.GetAuthorizeCodeSession(ctx, code, &fakeSession{})
	require.ErrorIs(t, err, fosite.ErrInvalidatedAuthorizeCode)
}

func newTestRequest(
	t *testing.T, client fosite.Client, requestID string,
) *fosite.Request {
	t.Helper()
	//nolint:exhaustruct //ExpiresAt is unused by these tests
	session := &fakeSession{Subject: uuid.NewString()}
	//nolint:exhaustruct //other fosite.Request fields are optional for these tests
	return &fosite.Request{
		ID:          requestID,
		RequestedAt: time.Now(),
		Client:      client,
		Session:     session,
	}
}

func TestStore_RefreshTokenSession_RoundTrip(t *testing.T) {
	store, db := newTestStore(t)
	ctx := context.Background()

	//nolint:exhaustruct //ClientName is optional
	client, err := oauth2as.RegisterClient(ctx, db, oauth2as.ClientMetadata{
		RedirectURIs: []string{"https://example.com/callback"},
	})
	require.NoError(t, err)

	requestID := uuid.NewString()
	request := newTestRequest(t, client, requestID)

	signature := uuid.NewString()
	require.NoError(
		t, store.CreateRefreshTokenSession(ctx, signature, "access-sig", request),
	)

	//nolint:exhaustruct //populated by GetRefreshTokenSession's json.Unmarshal
	got, err := store.GetRefreshTokenSession(ctx, signature, &fakeSession{})
	require.NoError(t, err)
	assert.Equal(t, request.Session.GetSubject(), got.GetSession().GetSubject())
	assert.Equal(t, client.ID, got.GetClient().GetID())

	require.NoError(t, store.DeleteRefreshTokenSession(ctx, signature))
	//nolint:exhaustruct //populated by GetRefreshTokenSession's json.Unmarshal
	_, err = store.GetRefreshTokenSession(ctx, signature, &fakeSession{})
	require.ErrorIs(t, err, fosite.ErrNotFound)
}

func TestStore_RotateRefreshToken(t *testing.T) {
	store, db := newTestStore(t)
	ctx := context.Background()

	//nolint:exhaustruct //ClientName is optional
	client, err := oauth2as.RegisterClient(ctx, db, oauth2as.ClientMetadata{
		RedirectURIs: []string{"https://example.com/callback"},
	})
	require.NoError(t, err)

	requestID := uuid.NewString()
	request := newTestRequest(t, client, requestID)

	signature := uuid.NewString()
	require.NoError(
		t, store.CreateRefreshTokenSession(ctx, signature, "access-sig", request),
	)

	require.NoError(t, store.RotateRefreshToken(ctx, requestID, signature))

	// A rotated-out refresh token is invalidated (active = false) and
	// stamped with rotated_at = now(). Reused well outside the reuse grace
	// period, GetRefreshTokenSession surfaces fosite.ErrInactiveToken — the
	// sentinel fosite's own RefreshTokenGrantHandler checks for to detect
	// refresh-token reuse and revoke the whole token family.
	_, err = db.Exec(ctx, `
		UPDATE auth.oauth2_refresh_tokens
		SET rotated_at = now() - interval '1 minute' WHERE signature = $1
	`, signature)
	require.NoError(t, err)
	//nolint:exhaustruct //populated by GetRefreshTokenSession's json.Unmarshal
	_, err = store.GetRefreshTokenSession(ctx, signature, &fakeSession{})
	require.ErrorIs(t, err, fosite.ErrInactiveToken)

	// Rotating a signature that doesn't match the given requestID is a
	// silent no-op (matches zero rows, still no error).
	require.NoError(t, store.RotateRefreshToken(ctx, "does-not-exist", signature))
}

func TestStore_RefreshTokenSession_ReuseGracePeriod(t *testing.T) {
	store, db := newTestStore(t)
	ctx := context.Background()

	//nolint:exhaustruct //ClientName is optional
	client, err := oauth2as.RegisterClient(ctx, db, oauth2as.ClientMetadata{
		RedirectURIs: []string{"https://example.com/callback"},
	})
	require.NoError(t, err)

	requestID := uuid.NewString()
	request := newTestRequest(t, client, requestID)

	signature := uuid.NewString()
	require.NoError(
		t, store.CreateRefreshTokenSession(ctx, signature, "access-sig", request),
	)
	require.NoError(t, store.RotateRefreshToken(ctx, requestID, signature))

	// Reused immediately after rotation (a client retrying a request whose
	// response it never saw), the token is still accepted as if active —
	// no fosite.ErrInactiveToken, no theft-response revocation.
	//nolint:exhaustruct //populated by GetRefreshTokenSession's json.Unmarshal
	got, err := store.GetRefreshTokenSession(ctx, signature, &fakeSession{})
	require.NoError(t, err)
	assert.Equal(t, requestID, got.GetID())
}

func TestStore_RevokeRefreshToken(t *testing.T) {
	store, db := newTestStore(t)
	ctx := context.Background()

	//nolint:exhaustruct //ClientName is optional
	client, err := oauth2as.RegisterClient(ctx, db, oauth2as.ClientMetadata{
		RedirectURIs: []string{"https://example.com/callback"},
	})
	require.NoError(t, err)

	requestID := uuid.NewString()
	request := newTestRequest(t, client, requestID)

	signature := uuid.NewString()
	require.NoError(
		t, store.CreateRefreshTokenSession(ctx, signature, "access-sig", request),
	)

	require.NoError(t, store.RevokeRefreshToken(ctx, requestID))

	//nolint:exhaustruct //populated by GetRefreshTokenSession's json.Unmarshal
	_, err = store.GetRefreshTokenSession(ctx, signature, &fakeSession{})
	require.ErrorIs(t, err, fosite.ErrNotFound)

	// Revoking an unknown request ID is a no-op, not an error.
	require.NoError(t, store.RevokeRefreshToken(ctx, "does-not-exist"))
}

func TestStore_RevokeAccessToken(t *testing.T) {
	store, db := newTestStore(t)
	ctx := context.Background()

	//nolint:exhaustruct //ClientName is optional
	client, err := oauth2as.RegisterClient(ctx, db, oauth2as.ClientMetadata{
		RedirectURIs: []string{"https://example.com/callback"},
	})
	require.NoError(t, err)

	requestID := uuid.NewString()
	//nolint:exhaustruct //ExpiresAt is set explicitly below via SetExpiresAt
	session := &fakeSession{Subject: uuid.NewString()}
	session.SetExpiresAt(fosite.AccessToken, time.Now().Add(time.Hour))
	//nolint:exhaustruct //other fosite.Request fields are optional for this test
	request := &fosite.Request{
		ID:          requestID,
		RequestedAt: time.Now(),
		Client:      client,
		Session:     session,
	}

	signature := uuid.NewString()
	require.NoError(t, store.CreateAccessTokenSession(ctx, signature, request))

	require.NoError(t, store.RevokeAccessToken(ctx, requestID))

	//nolint:exhaustruct //populated by GetAccessTokenSession's json.Unmarshal
	_, err = store.GetAccessTokenSession(ctx, signature, &fakeSession{})
	require.ErrorIs(t, err, fosite.ErrNotFound)

	// Revoking an unknown request ID is a no-op, not an error.
	require.NoError(t, store.RevokeAccessToken(ctx, "does-not-exist"))
}

// TestStore_ClientAssertionJWT_Stubs covers ClientAssertionJWTValid/
// SetClientAssertionJWT: no-op stubs backing the private_key_jwt
// client-auth method, which this public-client-only server never uses, kept
// only to satisfy fosite's ClientManager interface.
func TestStore_ClientAssertionJWT_Stubs(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.ClientAssertionJWTValid(ctx, "any-jti"))
	require.NoError(t, store.SetClientAssertionJWT(ctx, "any-jti", time.Now()))
}

func TestStore_PKCERequestSession_RoundTrip(t *testing.T) {
	store, db := newTestStore(t)
	ctx := context.Background()

	//nolint:exhaustruct //ClientName is optional
	client, err := oauth2as.RegisterClient(ctx, db, oauth2as.ClientMetadata{
		RedirectURIs: []string{"https://example.com/callback"},
	})
	require.NoError(t, err)

	request := newTestRequest(t, client, uuid.NewString())

	signature := uuid.NewString()
	require.NoError(t, store.CreatePKCERequestSession(ctx, signature, request))

	//nolint:exhaustruct //populated by GetPKCERequestSession's json.Unmarshal
	got, err := store.GetPKCERequestSession(ctx, signature, &fakeSession{})
	require.NoError(t, err)
	assert.Equal(t, client.ID, got.GetClient().GetID())

	// Re-creating with the same signature upserts rather than conflicting.
	require.NoError(t, store.CreatePKCERequestSession(ctx, signature, request))

	require.NoError(t, store.DeletePKCERequestSession(ctx, signature))
	//nolint:exhaustruct //populated by GetPKCERequestSession's json.Unmarshal
	_, err = store.GetPKCERequestSession(ctx, signature, &fakeSession{})
	require.ErrorIs(t, err, fosite.ErrNotFound)
}
