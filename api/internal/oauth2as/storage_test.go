package oauth2as_test

import (
	"context"
	"crypto/rsa"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ory/fosite"
	"github.com/ory/fosite/handler/openid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/oauth2as"
	"tools.xdoubleu.com/internal/testhelper"
)

func newTestStore(t *testing.T) (*oauth2as.Store, *pgxpool.Pool) {
	t.Helper()
	db := testhelper.ConnectTestDB(testhelper.NewTestConfig().DBDsn)
	t.Cleanup(db.Close)
	return oauth2as.NewStore(db), db
}

// testOIDCKeyOnce shares one generated RSA key across tests (generation is slow).
//
//nolint:gochecknoglobals // one shared RSA signing key across the package's tests
var (
	testOIDCKeyOnce sync.Once
	testOIDCKeyVal  *rsa.PrivateKey
)

func testOIDCKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	testOIDCKeyOnce.Do(func() {
		key, _, err := oauth2as.LoadOrGenerateOIDCKey("")
		require.NoError(t, err)
		testOIDCKeyVal = key
	})
	return testOIDCKeyVal
}

// newTestProvider builds the provider as cmd/api does.
func newTestProvider(
	t *testing.T, cfg config.Config, store *oauth2as.Store,
) fosite.OAuth2Provider {
	t.Helper()
	return oauth2as.NewProvider(cfg, store, testOIDCKey(t))
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

func TestStore_GetRefreshTokenSession_UnmarshalError(t *testing.T) {
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

	// Valid JSON of the wrong shape fails json.Unmarshal into persistedRequest.
	_, err = db.Exec(ctx, `
		UPDATE auth.oauth2_refresh_tokens SET request = '"not-an-object"'::jsonb
		WHERE signature = $1
	`, signature)
	require.NoError(t, err)

	//nolint:exhaustruct //populated by GetRefreshTokenSession's json.Unmarshal
	_, err = store.GetRefreshTokenSession(ctx, signature, &fakeSession{})
	require.Error(t, err)
}

func TestStore_GetRefreshTokenSession_UnknownClient(t *testing.T) {
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

	// The embedded client_id no longer exists, so GetClient fails.
	_, err = db.Exec(ctx, `
		UPDATE auth.oauth2_refresh_tokens
		SET request = jsonb_set(request, '{client_id}', '"does-not-exist"')
		WHERE signature = $1
	`, signature)
	require.NoError(t, err)

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

	// Past the grace period, a rotated-out token yields fosite.ErrInactiveToken,
	// which fosite uses to detect reuse.
	_, err = db.Exec(ctx, `
		UPDATE auth.oauth2_refresh_tokens
		SET rotated_at = now() - interval '1 minute' WHERE signature = $1
	`, signature)
	require.NoError(t, err)
	//nolint:exhaustruct //populated by GetRefreshTokenSession's json.Unmarshal
	_, err = store.GetRefreshTokenSession(ctx, signature, &fakeSession{})
	require.ErrorIs(t, err, fosite.ErrInactiveToken)

	// A signature not matching requestID is a silent no-op.
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

	// Reused right after rotation, the token is still accepted.
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

	require.NoError(t, store.RevokeAccessToken(ctx, "does-not-exist"))
}

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

	require.NoError(t, store.CreatePKCERequestSession(ctx, signature, request))

	require.NoError(t, store.DeletePKCERequestSession(ctx, signature))
	//nolint:exhaustruct //populated by GetPKCERequestSession's json.Unmarshal
	_, err = store.GetPKCERequestSession(ctx, signature, &fakeSession{})
	require.ErrorIs(t, err, fosite.ErrNotFound)
}

func TestStore_OpenIDConnectSession_RoundTrip(t *testing.T) {
	store, db := newTestStore(t)
	ctx := context.Background()

	//nolint:exhaustruct //ClientName is optional
	client, err := oauth2as.RegisterClient(ctx, db, oauth2as.ClientMetadata{
		RedirectURIs: []string{"https://example.com/callback"},
	})
	require.NoError(t, err)

	request := newTestRequest(t, client, uuid.NewString())
	code := uuid.NewString()

	require.NoError(t, store.CreateOpenIDConnectSession(ctx, code, request))
	require.NoError(t, store.CreateOpenIDConnectSession(ctx, code, request))

	//nolint:exhaustruct //populated by GetOpenIDConnectSession's json.Unmarshal
	got, err := store.GetOpenIDConnectSession(ctx, code, &fosite.Request{
		Session: &fakeSession{},
	})
	require.NoError(t, err)
	assert.Equal(t, client.ID, got.GetClient().GetID())

	require.NoError(t, store.DeleteOpenIDConnectSession(ctx, code))
	//nolint:exhaustruct //populated by GetOpenIDConnectSession's json.Unmarshal
	_, err = store.GetOpenIDConnectSession(ctx, code, &fosite.Request{
		Session: &fakeSession{},
	})
	require.ErrorIs(t, err, openid.ErrNoSessionFound)
}
