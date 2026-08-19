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
