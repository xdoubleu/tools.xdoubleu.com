package oauthconn_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/database"
	"golang.org/x/oauth2"

	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/oauthconn"
)

type stubStore struct {
	tok          *oauth2.Token
	conn         *models.OAuthConnection
	getErr       error
	updateCalls  int
	updatedToken *oauth2.Token
}

func (s *stubStore) Get(
	context.Context, models.OAuthProvider,
) (*oauth2.Token, *models.OAuthConnection, error) {
	if s.getErr != nil {
		return nil, nil, s.getErr
	}
	return s.tok, s.conn, nil
}

func (s *stubStore) UpdateToken(
	_ context.Context, _ models.OAuthProvider, tok *oauth2.Token,
) error {
	s.updateCalls++
	s.updatedToken = tok
	return nil
}

func TestNewTokenFunc_NotConnected(t *testing.T) {
	store := &stubStore{ //nolint:exhaustruct // other fields unused in test
		getErr: database.ErrResourceNotFound,
	}
	fn := oauthconn.NewTokenFunc(
		store,
		models.OAuthProviderGithub,
		&oauth2.Config{}, //nolint:exhaustruct // other fields unused in test
	)

	_, err := fn(context.Background())
	assert.ErrorIs(t, err, oauthconn.ErrNotConnected)
}

func TestNewTokenFunc_NonExpiringToken_NoRefreshCall(t *testing.T) {
	store := &stubStore{ //nolint:exhaustruct // other fields unused in test
		tok: &oauth2.Token{ //nolint:exhaustruct // other fields unused in test
			AccessToken: "abc",
		},
	}
	fn := oauthconn.NewTokenFunc(
		store,
		models.OAuthProviderGithub,
		&oauth2.Config{}, //nolint:exhaustruct // other fields unused in test
	)

	token, err := fn(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "abc", token)
	assert.Equal(
		t,
		0,
		store.updateCalls,
		"a still-valid token must not be persisted again",
	)
}

func TestNewTokenFunc_ExpiredToken_RefreshesAndPersists(t *testing.T) {
	srv := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(
				`{"access_token":"new-token","token_type":"Bearer","expires_in":3600}`,
			))
		}),
	)
	defer srv.Close()

	store := &stubStore{ //nolint:exhaustruct // other fields unused in test
		tok: &oauth2.Token{ //nolint:exhaustruct // other fields unused in test
			AccessToken:  "old-token",
			RefreshToken: "refresh-token",
			Expiry:       time.Now().Add(-time.Hour),
		},
	}
	conf := &oauth2.Config{ //nolint:exhaustruct // other fields unused in test
		//nolint:exhaustruct // other fields unused in test
		Endpoint: oauth2.Endpoint{TokenURL: srv.URL},
	}
	fn := oauthconn.NewTokenFunc(store, models.OAuthProviderGithub, conf)

	token, err := fn(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "new-token", token)
	assert.Equal(t, 1, store.updateCalls)
	require.NotNil(t, store.updatedToken)
	assert.Equal(t, "new-token", store.updatedToken.AccessToken)
}
