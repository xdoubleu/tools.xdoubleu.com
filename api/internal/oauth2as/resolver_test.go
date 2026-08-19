package oauth2as_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ory/fosite"
	"github.com/ory/fosite/compose"
	fositeoauth2 "github.com/ory/fosite/handler/oauth2"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/oauth2as"
	"tools.xdoubleu.com/internal/testhelper"
)

// hmacStrategyFor builds a bare oauth2.HMACSHAStrategy sharing the test
// config's GlobalSecret, so tests can mint access tokens directly against
// the store without driving a full HTTP authorize+token exchange — used to
// reach resolver/store states (expired, no-subject) a real flow can't
// produce without the hour-long AccessTokenLifespan baked into server.go
// actually elapsing.
func hmacStrategyFor(t *testing.T) fositeoauth2.CoreStrategy {
	t.Helper()
	cfg := testhelper.NewTestConfig()
	//nolint:exhaustruct //remaining Config fields use library defaults
	fc := &fosite.Config{GlobalSecret: []byte(cfg.OAuthHMACSecret)}
	return compose.NewOAuth2HMACStrategy(fc)
}

func TestResolveAccessToken_ValidToken(t *testing.T) {
	store, db := newTestStore(t)
	cfg := testhelper.NewTestConfig()
	provider := oauth2as.NewProvider(cfg, store)
	resolver := oauth2as.NewTokenResolver(provider)

	//nolint:exhaustruct //ClientName is optional
	client, err := oauth2as.RegisterClient(
		context.Background(),
		db,
		oauth2as.ClientMetadata{
			RedirectURIs: []string{"https://example.com/callback"},
		},
	)
	require.NoError(t, err)

	userID := uuid.NewString()
	//nolint:exhaustruct //Username/Extra are unused by this test
	session := &fosite.DefaultSession{
		Subject: userID,
		//nolint:exhaustive //only AccessToken expiry matters for this test
		ExpiresAt: map[fosite.TokenType]time.Time{
			fosite.AccessToken: time.Now().Add(time.Hour),
		},
	}
	//nolint:exhaustruct //other fosite.Request fields are optional for this test
	request := &fosite.Request{
		ID:          uuid.NewString(),
		RequestedAt: time.Now(),
		Client:      client,
		Session:     session,
	}

	strategy := hmacStrategyFor(t)
	token, signature, err := strategy.GenerateAccessToken(context.Background(), request)
	require.NoError(t, err)
	require.NoError(
		t,
		store.CreateAccessTokenSession(context.Background(), signature, request),
	)

	gotUserID, err := resolver.ResolveAccessToken(context.Background(), token)
	require.NoError(t, err)
	require.Equal(t, userID, gotUserID)
}

func TestResolveAccessToken_GarbageToken(t *testing.T) {
	store, _ := newTestStore(t)
	cfg := testhelper.NewTestConfig()
	provider := oauth2as.NewProvider(cfg, store)
	resolver := oauth2as.NewTokenResolver(provider)

	_, err := resolver.ResolveAccessToken(context.Background(), "not-a-real-token")
	require.Error(t, err)
}

func TestResolveAccessToken_ExpiredToken(t *testing.T) {
	store, db := newTestStore(t)
	cfg := testhelper.NewTestConfig()
	provider := oauth2as.NewProvider(cfg, store)
	resolver := oauth2as.NewTokenResolver(provider)

	//nolint:exhaustruct //ClientName is optional
	client, err := oauth2as.RegisterClient(
		context.Background(),
		db,
		oauth2as.ClientMetadata{
			RedirectURIs: []string{"https://example.com/callback"},
		},
	)
	require.NoError(t, err)

	//nolint:exhaustruct //Username/Extra are unused by this test
	session := &fosite.DefaultSession{
		Subject: uuid.NewString(),
		//nolint:exhaustive //only AccessToken expiry matters for this test
		ExpiresAt: map[fosite.TokenType]time.Time{
			fosite.AccessToken: time.Now().Add(-time.Hour),
		},
	}
	//nolint:exhaustruct //other fosite.Request fields are optional for this test
	request := &fosite.Request{
		ID:          uuid.NewString(),
		RequestedAt: time.Now(),
		Client:      client,
		Session:     session,
	}

	strategy := hmacStrategyFor(t)
	token, signature, err := strategy.GenerateAccessToken(context.Background(), request)
	require.NoError(t, err)
	require.NoError(
		t,
		store.CreateAccessTokenSession(context.Background(), signature, request),
	)

	_, err = resolver.ResolveAccessToken(context.Background(), token)
	require.Error(t, err)
}

func TestResolveAccessToken_NoSubject(t *testing.T) {
	store, db := newTestStore(t)
	cfg := testhelper.NewTestConfig()
	provider := oauth2as.NewProvider(cfg, store)
	resolver := oauth2as.NewTokenResolver(provider)

	//nolint:exhaustruct //ClientName is optional
	client, err := oauth2as.RegisterClient(
		context.Background(),
		db,
		oauth2as.ClientMetadata{
			RedirectURIs: []string{"https://example.com/callback"},
		},
	)
	require.NoError(t, err)

	//nolint:exhaustruct //Subject/Username/Extra are unused by this test
	session := &fosite.DefaultSession{
		//nolint:exhaustive //only AccessToken expiry matters for this test
		ExpiresAt: map[fosite.TokenType]time.Time{
			fosite.AccessToken: time.Now().Add(time.Hour),
		},
	}
	//nolint:exhaustruct //other fosite.Request fields are optional for this test
	request := &fosite.Request{
		ID:          uuid.NewString(),
		RequestedAt: time.Now(),
		Client:      client,
		Session:     session,
	}

	strategy := hmacStrategyFor(t)
	token, signature, err := strategy.GenerateAccessToken(context.Background(), request)
	require.NoError(t, err)
	require.NoError(
		t,
		store.CreateAccessTokenSession(context.Background(), signature, request),
	)

	_, err = resolver.ResolveAccessToken(context.Background(), token)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no subject")
}

func TestResolveAccessToken_RevokedToken(t *testing.T) {
	store, db := newTestStore(t)
	cfg := testhelper.NewTestConfig()
	provider := oauth2as.NewProvider(cfg, store)
	resolver := oauth2as.NewTokenResolver(provider)

	//nolint:exhaustruct //ClientName is optional
	client, err := oauth2as.RegisterClient(
		context.Background(),
		db,
		oauth2as.ClientMetadata{
			RedirectURIs: []string{"https://example.com/callback"},
		},
	)
	require.NoError(t, err)

	requestID := uuid.NewString()
	//nolint:exhaustruct //Username/Extra are unused by this test
	session := &fosite.DefaultSession{
		Subject: uuid.NewString(),
		//nolint:exhaustive //only AccessToken expiry matters for this test
		ExpiresAt: map[fosite.TokenType]time.Time{
			fosite.AccessToken: time.Now().Add(time.Hour),
		},
	}
	//nolint:exhaustruct //other fosite.Request fields are optional for this test
	request := &fosite.Request{
		ID:          requestID,
		RequestedAt: time.Now(),
		Client:      client,
		Session:     session,
	}

	strategy := hmacStrategyFor(t)
	token, signature, err := strategy.GenerateAccessToken(context.Background(), request)
	require.NoError(t, err)
	require.NoError(
		t,
		store.CreateAccessTokenSession(context.Background(), signature, request),
	)

	_, err = resolver.ResolveAccessToken(context.Background(), token)
	require.NoError(t, err)

	require.NoError(t, store.RevokeAccessToken(context.Background(), requestID))

	_, err = resolver.ResolveAccessToken(context.Background(), token)
	require.Error(t, err)
}
