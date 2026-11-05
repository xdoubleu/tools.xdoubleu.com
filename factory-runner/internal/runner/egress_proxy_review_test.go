package runner_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
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

func TestTheBoardRejectingTheTokenTellsTheTokenSource(t *testing.T) {
	var rejected int
	p, up := proxyFor(t, func(c *runner.EgressConfig) {
		c.MCPRejected = func() { rejected++ }
	})
	up.srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	rec := do(t, p, http.MethodPost, "/mcp", "{}", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, 1, rejected)

	rec = do(t, p, http.MethodPost, "/anthropic/v1/messages", "{}", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, 1, rejected, "another route's 401 is the VM's own business")
}

func TestPathsReachTheUpstreamAsSentAndOnlyDotSegmentsAreRefused(t *testing.T) {
	p, up := proxyFor(t, nil)
	for target, want := range map[string]string{
		"/anthropic/v1/messages/": "/v1/messages/",
		"/anthropic/v1//x":        "/v1//x",
		"/anthropic/v1..beta/x":   "/v1..beta/x",
		"/anthropic/a%20b":        "/a b",
		"/openrouter/v1/models":   "/api/v1/models",
	} {
		up.got = nil
		rec := do(t, p, http.MethodGet, target, "", nil)
		require.Equal(t, http.StatusOK, rec.Code, target)
		assert.Equal(t, want, up.got.URL.Path, target)
	}
	up.got = nil
	for _, target := range []string{
		"/anthropic/./x", "/anthropic/a/../x", "/anthropic/..",
	} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.URL.Path = target
		rec := httptest.NewRecorder()
		p.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code, target)
	}
	assert.Nil(t, up.got)
}

func TestWhyACredentialFailedIsLoggedButNeverSentToTheVM(t *testing.T) {
	var logs bytes.Buffer
	p, _ := proxyFor(t, func(c *runner.EgressConfig) {
		c.MCPToken = func(context.Context) (string, error) {
			return "", errors.New("board token: status 401")
		}
		c.Log = slog.New(slog.NewTextHandler(&logs, nil))
	})

	rec := do(t, p, http.MethodPost, "/mcp", "{}", nil)

	assert.Equal(t, http.StatusBadGateway, rec.Code)
	assert.NotContains(t, rec.Body.String(), "401")
	assert.Contains(t, logs.String(), "board token: status 401")
}

func TestTheProxyAnswersOnlyOnItsGatewayAddress(t *testing.T) {
	gateway := firstNonLoopbackIPv4(t)
	p, _ := proxyFor(t, nil)
	stop, addr, err := p.Listen(gateway, 0)
	require.NoError(t, err)
	defer stop()
	_, port, err := net.SplitHostPort(addr)
	require.NoError(t, err)

	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", port), time.Second)
	if err == nil {
		_ = conn.Close()
	}
	require.Error(t, err, "a wildcard listener would also answer on loopback")
}
