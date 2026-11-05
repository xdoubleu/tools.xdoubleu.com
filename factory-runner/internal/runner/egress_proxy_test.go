package runner_test

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

// upstream records what reached it and answers 200 "up".
type upstream struct {
	srv  *httptest.Server
	got  *http.Request
	body string
}

func newUpstream(t *testing.T) *upstream {
	t.Helper()
	u := &upstream{srv: nil, got: nil, body: ""}
	handler := func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		u.got, u.body = r.Clone(context.Background()), string(b)
		w.Header().Set("Set-Cookie", "planted=1")
		w.Header().Set("X-Seen", "yes")
		_, _ = io.WriteString(w, "up")
	}
	u.srv = httptest.NewServer(http.HandlerFunc(handler))
	t.Cleanup(u.srv.Close)
	return u
}

func proxyFor(
	t *testing.T,
	mutate func(*runner.EgressConfig),
) (*runner.EgressProxy, *upstream) {
	t.Helper()
	up := newUpstream(t)
	cfg := runner.EgressConfig{
		Server:        up.srv.URL,
		MCPToken:      func(context.Context) (string, error) { return "mcp-token", nil },
		OpenRouterKey: func() (string, error) { return "real-or-key", nil },
		Transport:     nil,
		Anthropic:     up.srv.URL,
		OpenRouter:    up.srv.URL + "/api",
		Log:           nil,
	}
	if mutate != nil {
		mutate(&cfg)
	}
	p, err := runner.NewEgressProxy(cfg)
	require.NoError(t, err)
	return p, up
}

func do(
	t *testing.T,
	p http.Handler,
	method, target, body string,
	hdr map[string]string,
) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)
	return rec
}

func TestAnthropicTrafficPassesThroughWithTheVMsOwnCredential(t *testing.T) {
	p, up := proxyFor(t, nil)

	rec := do(t, p, http.MethodPost, "/anthropic/v1/messages?beta=1", `{"a":1}`,
		map[string]string{
			"Authorization":       "Bearer vm-setup-token",
			"Anthropic-Version":   "2023-06-01",
			"Cookie":              "session=secret",
			"Proxy-Authorization": "x",
		})

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "up", rec.Body.String())
	assert.Equal(t, "/v1/messages", up.got.URL.Path)
	assert.Equal(t, "beta=1", up.got.URL.RawQuery)
	assert.Equal(t, `{"a":1}`, up.body)
	assert.Equal(t, "Bearer vm-setup-token", up.got.Header.Get("Authorization"))
	assert.Equal(t, "2023-06-01", up.got.Header.Get("Anthropic-Version"))
	assert.Empty(t, up.got.Header.Get("Cookie"))
	assert.Empty(t, up.got.Header.Get("Proxy-Authorization"))
	assert.Equal(t, "yes", rec.Header().Get("X-Seen"))
	assert.Empty(t, rec.Header().Get("Set-Cookie"), "an upstream cannot plant a cookie")
}

func TestOpenRouterGetsTheRealKeyInPlaceOfTheVMsPlaceholder(t *testing.T) {
	p, up := proxyFor(t, nil)

	rec := do(t, p, http.MethodPost, "/openrouter/v1/chat/completions", "{}",
		map[string]string{
			"Authorization": "Bearer replaced-by-the-host-proxy",
			"X-Api-Key":     "also-a-placeholder",
		})

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "/api/v1/chat/completions", up.got.URL.Path)
	assert.Equal(t, "Bearer real-or-key", up.got.Header.Get("Authorization"))
	assert.Empty(t, up.got.Header.Get("X-Api-Key"))
}

func TestTheBoardGetsTheJobsTokenOnItsOneEndpoint(t *testing.T) {
	p, up := proxyFor(t, nil)

	rec := do(t, p, http.MethodPost, "/mcp", `{"jsonrpc":"2.0"}`,
		map[string]string{"Authorization": "Bearer from-the-vm"})
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "/apps/mcp", up.got.URL.Path)
	assert.Equal(t, "Bearer mcp-token", up.got.Header.Get("Authorization"))
	assert.Equal(t, `{"jsonrpc":"2.0"}`, up.body)

	up.got = nil
	for _, p2 := range []string{"/mcp/", "/mcp/other", "/mcpx"} {
		rec = do(t, p, http.MethodPost, p2, "{}", nil)
		assert.Equal(t, http.StatusForbidden, rec.Code, p2)
	}
	assert.Nil(t, up.got, "nothing else reached the board")
}

func TestEverythingElseIsRefusedAndNeverReachesAnUpstream(t *testing.T) {
	p, up := proxyFor(t, nil)

	for _, target := range []string{
		"/", "/other", "/anthropic", "/openrouterx/v1", "/v1/messages",
	} {
		rec := do(t, p, http.MethodGet, target, "", nil)
		assert.Equal(t, http.StatusForbidden, rec.Code, target)
	}
	for _, m := range []string{http.MethodPut, http.MethodPatch, http.MethodConnect} {
		assert.Equal(t, http.StatusMethodNotAllowed,
			do(t, p, m, "/anthropic/v1/messages", "", nil).Code, m)
	}
	assert.Nil(t, up.got)
}

func TestAPathThatClimbsOutOfItsRouteIsRefused(t *testing.T) {
	p, up := proxyFor(t, nil)

	for _, target := range []string{
		"/anthropic/../openrouter/v1", "/anthropic/v1/../../x", "/openrouter/%2e%2e/x",
		"/anthropic/v1%2fmessages",
	} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.URL = &url.URL{Path: target}
		if strings.Contains(target, "%") {
			req.URL.Path, _ = url.PathUnescape(target)
			req.URL.RawPath = target
		}
		rec := httptest.NewRecorder()
		p.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code, target)
	}
	assert.Nil(t, up.got)
}

func TestARouteWithoutItsCredentialSourceIsLeftOut(t *testing.T) {
	p, up := proxyFor(t, func(c *runner.EgressConfig) {
		c.OpenRouterKey = nil
		c.MCPToken = nil
	})
	assert.Equal(t, http.StatusForbidden,
		do(t, p, http.MethodPost, "/openrouter/v1/x", "{}", nil).Code)
	assert.Equal(t, http.StatusForbidden,
		do(t, p, http.MethodPost, "/mcp", "{}", nil).Code)
	assert.Equal(t, http.StatusOK,
		do(t, p, http.MethodPost, "/anthropic/v1/x", "{}", nil).Code)
	assert.NotNil(t, up.got)
}

func TestACredentialThatCannotBeFetchedFailsTheRequestWithoutLeakingWhy(t *testing.T) {
	p, up := proxyFor(t, func(c *runner.EgressConfig) {
		c.MCPToken = func(context.Context) (string, error) {
			return "", errors.New("secret detail: client-secret-123")
		}
		c.OpenRouterKey = func() (string, error) { return "", errors.New("no key") }
	})
	for _, target := range []string{"/mcp", "/openrouter/v1/x"} {
		rec := do(t, p, http.MethodPost, target, "{}", nil)
		assert.Equal(t, http.StatusBadGateway, rec.Code, target)
		assert.NotContains(t, rec.Body.String(), "secret")
	}
	assert.Nil(t, up.got, "no request goes out without its credential")
}

func TestAnUpstreamThatIsDownIsAGenericBadGateway(t *testing.T) {
	p, _ := proxyFor(t, func(c *runner.EgressConfig) {
		c.Anthropic = "http://127.0.0.1:1" // nothing listens
	})
	rec := do(t, p, http.MethodPost, "/anthropic/v1/messages", "{}",
		map[string]string{"Authorization": "Bearer tok"})
	assert.Equal(t, http.StatusBadGateway, rec.Code)
	assert.NotContains(t, rec.Body.String(), "127.0.0.1")
	assert.NotContains(t, rec.Body.String(), "tok")
}

func TestResponsesStreamAsTheyArrive(t *testing.T) {
	release := make(chan struct{})
	events := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: one\n\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		<-release
		_, _ = io.WriteString(w, "data: two\n\n")
	}
	up := httptest.NewServer(http.HandlerFunc(events))
	defer up.Close()
	p, err := runner.NewEgressProxy(runner.EgressConfig{
		Server: "", MCPToken: nil, OpenRouterKey: nil, Transport: nil,
		Anthropic: up.URL, OpenRouter: "", Log: nil,
	})
	require.NoError(t, err)
	front := httptest.NewServer(p)
	defer front.Close()

	resp, err := http.Post(front.URL+"/anthropic/v1/messages", "application/json",
		strings.NewReader("{}"))
	require.NoError(t, err)
	defer resp.Body.Close()
	first := make([]byte, len("data: one\n\n"))
	_, err = io.ReadFull(resp.Body, first)
	require.NoError(t, err, "the first event arrives while the upstream is still open")
	assert.Equal(t, "data: one\n\n", string(first))
	close(release)
	rest, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "data: two\n\n", string(rest))
}

func TestABadUpstreamBaseIsRejectedWhenBuildingTheProxy(t *testing.T) {
	_, err := runner.NewEgressProxy(runner.EgressConfig{
		Server: "", MCPToken: nil, OpenRouterKey: nil, Transport: nil,
		Anthropic: "not a url", OpenRouter: "", Log: nil,
	})
	require.ErrorContains(t, err, "bad upstream")
}

func TestListenRefusesAnythingButAJobGateway(t *testing.T) {
	p, _ := proxyFor(t, nil)
	for _, gw := range []string{
		"0.0.0.0", "::", "", "127.0.0.1", "::1", "224.0.0.1", "nope",
	} {
		stop, addr, err := p.Listen(gw, 0)
		require.Error(t, err, gw)
		assert.Nil(t, stop)
		assert.Empty(t, addr)
	}
}

func TestListenServesOnTheGivenAddressOnly(t *testing.T) {
	gateway := firstNonLoopbackIPv4(t)
	p, up := proxyFor(t, nil)

	stop, addr, err := p.Listen(gateway, 0)
	require.NoError(t, err)
	host, _, err := net.SplitHostPort(addr)
	require.NoError(t, err)
	assert.Equal(t, gateway, host)

	resp, err := http.Post("http://"+addr+"/anthropic/v1/messages", "application/json",
		strings.NewReader("{}"))
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "/v1/messages", up.got.URL.Path)

	stop()
	_, err = http.Post("http://"+addr+"/anthropic/v1/messages", "application/json",
		strings.NewReader("{}"))
	require.Error(t, err, "stopped")
}

func TestListenFailsWhenTheAddressIsNotThisHosts(t *testing.T) {
	p, _ := proxyFor(t, nil)
	_, _, err := p.Listen("203.0.113.9", 0) // TEST-NET-3, never local
	require.ErrorContains(t, err, "egress proxy")
}

func firstNonLoopbackIPv4(t *testing.T) string {
	t.Helper()
	addrs, err := net.InterfaceAddrs()
	require.NoError(t, err)
	for _, a := range addrs {
		if ipn, ok := a.(*net.IPNet); ok && ipn.IP.To4() != nil && !ipn.IP.IsLoopback() {
			return ipn.IP.String()
		}
	}
	t.Skip("no non-loopback IPv4 address on this host")
	return ""
}

func TestTheHostInARequestNeverChoosesTheDestination(t *testing.T) {
	p, up := proxyFor(t, nil)

	rec := do(t, p, http.MethodGet, "http://evil.example/anthropic/v1/models", "", nil)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "/v1/models", up.got.URL.Path)
	assert.NotContains(t, up.got.Host, "evil")
}
