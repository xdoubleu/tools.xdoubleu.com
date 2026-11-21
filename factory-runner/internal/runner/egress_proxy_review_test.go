package runner_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

func TestARejectedTokenIsDroppedSoTheNextCallMintsAFreshOne(t *testing.T) {
	a := newAuthServer(t)
	src := a.source(nil)
	_, err := src.Token(context.Background())
	require.NoError(t, err)

	src.Invalidate()
	a.token = "tok-2"
	tok, err := src.Token(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "tok-2", tok)
	assert.Equal(t, 2, a.mints)
}

func TestAShortLivedTokenIsStillCachedForMostOfItsLife(t *testing.T) {
	a := newAuthServer(t)
	a.expiresIn = 30
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	src := a.source(func() time.Time { return now })

	_, err := src.Token(context.Background())
	require.NoError(t, err)
	now = now.Add(20 * time.Second)
	_, err = src.Token(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, a.mints, "a 30 s token is not re-minted on every call")

	now = now.Add(8 * time.Second)
	_, err = src.Token(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, a.mints)
}

func TestATokenEndpointOffTheConfiguredServerOrOffHTTPSIsRefused(t *testing.T) {
	for name, endpoint := range map[string]func(srvURL string) string{
		"another host": func(string) string { return "https://evil.example/token" },
		"not https": func(u string) string {
			return "ftp://" + strings.TrimPrefix(u, "http://")
		},
		"unparsable url": func(string) string { return "http://[::1" },
	} {
		var srv *httptest.Server
		metadata := func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]string{
				"token_endpoint": endpoint(srv.URL),
			})
		}
		srv = httptest.NewServer(http.HandlerFunc(metadata))
		src := &runner.MCPTokenSource{
			ServerURL: srv.URL, ClientID: "a", ClientSecret: "b", HTTP: nil, Now: nil,
		}
		_, err := src.Token(context.Background())
		require.ErrorContains(t, err, "token", name)
		assert.NotContains(t, err.Error(), "b\"", name)
		srv.Close()
	}
}
