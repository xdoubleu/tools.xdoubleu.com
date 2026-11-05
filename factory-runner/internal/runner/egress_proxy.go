package runner

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/netip"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

const (
	// EgressProxyPort is where the proxy listens on a job network's gateway.
	EgressProxyPort = 8118

	anthropicURL  = "https://api.anthropic.com"
	openRouterURL = "https://openrouter.ai/api"

	maxProxyBody      = 64 << 20
	proxyHeaderWait   = 15 * time.Second
	upstreamHeaderMax = 10 * time.Minute
	proxyStopWait     = 5 * time.Second
)

// EgressConfig says where the proxy sends traffic and which credentials it adds.
type EgressConfig struct {
	// Server is the API root, like Config.Server; the board's MCP endpoint is
	// <Server>/apps/mcp.
	Server string
	// MCPToken and OpenRouterKey return the credential to add. A nil one leaves
	// its route out: the VM then cannot reach that service at all.
	MCPToken      func(ctx context.Context) (string, error)
	OpenRouterKey func() (string, error)
	// Transport is how the upstream is reached; nil means a TLS default.
	Transport http.RoundTripper
	// Anthropic and OpenRouter override the upstream bases, for tests.
	Anthropic  string
	OpenRouter string
	Log        *slog.Logger
}

// route is one fixed destination. Nothing the VM sends can name another.
type route struct {
	name   string
	prefix string
	// exact, when set, matches only the prefix itself.
	exact  bool
	target *url.URL
	// auth, when set, replaces the request's Authorization with its result;
	// otherwise the request's own passes through.
	auth func(ctx context.Context) (string, error)
}

// EgressProxy is the only way out of a job's VM (docs/adr-0043): a handful of fixed
// routes to the model providers and the board, with the real credentials added here
// where the VM can't read them. It is not a general proxy: no URL comes from the VM.
type EgressProxy struct {
	routes []route
	rp     *httputil.ReverseProxy
	log    *slog.Logger
}

type authKey struct{}

// NewEgressProxy builds the proxy for one job.
func NewEgressProxy(cfg EgressConfig) (*EgressProxy, error) {
	p := &EgressProxy{routes: nil, rp: nil, log: cfg.Log}
	if p.log == nil {
		p.log = slog.New(slog.DiscardHandler)
	}
	add := func(name, prefix string, exact bool, base string,
		auth func(context.Context) (string, error),
	) error {
		target, err := url.Parse(base)
		if err != nil || target.Scheme == "" || target.Host == "" {
			return fmt.Errorf("egress route %s: bad upstream %q", name, base)
		}
		p.routes = append(p.routes, route{
			name: name, prefix: prefix, exact: exact, target: target, auth: auth,
		})
		return nil
	}
	if err := add("anthropic", "/anthropic/", false,
		orDefault(cfg.Anthropic, anthropicURL), nil); err != nil {
		return nil, err
	}
	if cfg.OpenRouterKey != nil {
		key := cfg.OpenRouterKey
		err := add("openrouter", "/openrouter/", false,
			orDefault(cfg.OpenRouter, openRouterURL),
			func(context.Context) (string, error) {
				k, err := key()
				return "Bearer " + k, err
			})
		if err != nil {
			return nil, err
		}
	}
	if cfg.MCPToken != nil && cfg.Server != "" {
		token := cfg.MCPToken
		err := add("mcp", "/mcp", true, strings.TrimRight(cfg.Server, "/")+"/apps/mcp",
			func(ctx context.Context) (string, error) {
				t, err := token(ctx)
				return "Bearer " + t, err
			})
		if err != nil {
			return nil, err
		}
	}
	transport := cfg.Transport
	if transport == nil {
		base, ok := http.DefaultTransport.(*http.Transport)
		if !ok {
			return nil, errors.New("no default transport")
		}
		t := base.Clone()
		t.ResponseHeaderTimeout = upstreamHeaderMax
		transport = t
	}
	p.rp = &httputil.ReverseProxy{ //nolint:exhaustruct // defaults for the rest
		Rewrite:        p.rewrite,
		Transport:      transport,
		FlushInterval:  -1, // stream events as they come
		ModifyResponse: dropCookies,
		ErrorHandler:   p.upstreamError,
	}
	return p, nil
}

// match finds the route for a request path and the path under it.
func (p *EgressProxy) match(requestPath string) (*route, string) {
	for i := range p.routes {
		r := &p.routes[i]
		switch {
		case r.exact && requestPath == r.prefix:
			return r, ""
		case !r.exact && strings.HasPrefix(requestPath, r.prefix):
			return r, strings.TrimPrefix(requestPath, strings.TrimSuffix(r.prefix, "/"))
		}
	}
	return nil, ""
}

// ServeHTTP implements http.Handler.
func (p *EgressProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodPost, http.MethodDelete, http.MethodHead:
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// A route's base is a prefix, so a path that climbs out of it is refused rather
	// than cleaned.
	if strings.Contains(r.URL.Path, "..") || strings.Contains(r.URL.RawPath, "%") {
		http.Error(w, "bad path", http.StatusBadRequest)
		return
	}
	rt, rest := p.match(r.URL.Path)
	if rt == nil {
		p.log.Warn("egress refused", "method", r.Method, "path", safePath(r.URL.Path))
		http.Error(w, "not allowed", http.StatusForbidden)
		return
	}
	ctx := r.Context()
	if rt.auth != nil {
		header, err := rt.auth(ctx)
		if err != nil || header == "Bearer " {
			p.log.Error("egress credential unavailable", "route", rt.name)
			http.Error(w, "credential unavailable", http.StatusBadGateway)
			return
		}
		ctx = context.WithValue(ctx, authKey{}, header)
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxProxyBody)
	r2 := r.WithContext(context.WithValue(ctx, routeKey{}, routed{route: rt, rest: rest}))
	p.rp.ServeHTTP(w, r2)
}

type routeKey struct{}

type routed struct {
	route *route
	rest  string
}

// rewrite points the outgoing request at the route's fixed upstream.
func (p *EgressProxy) rewrite(pr *httputil.ProxyRequest) {
	got, _ := pr.In.Context().Value(routeKey{}).(routed)
	rt := got.route
	out := pr.Out
	out.URL.Scheme = rt.target.Scheme
	out.URL.Host = rt.target.Host
	out.Host = rt.target.Host
	out.URL.Path = path.Join(rt.target.Path, got.rest)
	out.URL.RawPath = ""
	for _, h := range []string{
		"Cookie", "Proxy-Authorization", "Proxy-Connection", "Forwarded", "Via",
	} {
		out.Header.Del(h)
	}
	if header, ok := pr.In.Context().Value(authKey{}).(string); ok {
		out.Header.Del("X-Api-Key")
		out.Header.Set("Authorization", header)
	}
	p.log.Info("egress", "route", rt.name, "method", out.Method)
}

// dropCookies keeps an upstream from planting state in the VM.
func dropCookies(resp *http.Response) error {
	resp.Header.Del("Set-Cookie")
	return nil
}

// upstreamError answers for a failed upstream without echoing its error, which
// could carry a header.
func (p *EgressProxy) upstreamError(w http.ResponseWriter, r *http.Request, err error) {
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
		return
	}
	p.log.Warn("egress upstream failed", "path", safePath(r.URL.Path))
	http.Error(w, "upstream unavailable", http.StatusBadGateway)
}

// safePath is a request path fit for a log line.
func safePath(p string) string {
	const maxLogPath = 80
	p = strings.NewReplacer("\n", "", "\r", "").Replace(p)
	if len(p) > maxLogPath {
		return p[:maxLogPath]
	}
	return p
}

// Listen serves the proxy on the job network's gateway address, and only there: a
// wildcard address would also be reachable from the VM through the host's other
// services. The returned func stops it and the string is the bound address. Use it
// as ContainerSandbox.OnReady.
func (p *EgressProxy) Listen(gateway string, port int) (func(), string, error) {
	addr, err := netip.ParseAddr(gateway)
	if err != nil || addr.IsUnspecified() || addr.IsLoopback() || addr.IsMulticast() {
		return nil, "", fmt.Errorf("refusing to listen on %q", gateway)
	}
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp",
		net.JoinHostPort(gateway, strconv.Itoa(port)))
	if err != nil {
		return nil, "", fmt.Errorf("egress proxy: %w", err)
	}
	srv := &http.Server{
		Handler:           p,
		ReadHeaderTimeout: proxyHeaderWait,
	}
	go func() { _ = srv.Serve(ln) }()
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), proxyStopWait)
		defer cancel()
		if srv.Shutdown(ctx) != nil {
			_ = srv.Close()
		}
	}, ln.Addr().String(), nil
}
