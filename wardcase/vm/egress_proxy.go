package vm

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
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	// EgressProxyPort is where the proxy listens on a job network's gateway.
	EgressProxyPort = 8118

	anthropicURL  = "https://api.anthropic.com"
	openRouterURL = "https://openrouter.ai/api"
	openAIURL     = "https://api.openai.com"
	geminiURL     = "https://generativelanguage.googleapis.com"
	goProxyURL    = "https://proxy.golang.org"
	npmURL        = "https://registry.npmjs.org"
	rubyGemsURL   = "https://rubygems.org"
	nixCacheURL   = "https://cache.nixos.org"
	nixSrcURL     = "https://codeload.github.com"

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
	// MCPToken, OpenRouterKey and OpenAIKey return the credential to add. A nil one
	// leaves its route out: the VM then cannot reach that service at all.
	MCPToken      func(ctx context.Context) (string, error)
	OpenRouterKey func() (string, error)
	OpenAIKey     func() (string, error)
	// GeminiKey is Gemini's API key, added as its x-goog-api-key header.
	GeminiKey func() (string, error)
	// MCPServers are further MCP servers, each reached at /mcp-<name>.
	MCPServers []MCPServer
	// MCPRejected, when set, is called when the board answers 401, so a token it
	// revoked early is not served again.
	MCPRejected func()
	// Transport is how the upstream is reached; nil means a TLS default.
	Transport http.RoundTripper
	// Anthropic, OpenRouter and OpenAI override the upstream bases, for tests.
	Anthropic  string
	OpenRouter string
	OpenAI     string
	Gemini     string
	GoProxy    string
	NPM        string
	RubyGems   string
	// NixCache and NixSrc are the Nix binary cache and the GitHub tarball host a
	// flake's inputs come from.
	NixCache string
	NixSrc   string
	Log      *slog.Logger
}

// MCPServer is an MCP server the VM reaches through the proxy at /mcp-<Name>, with
// no credential of its own: the proxy adds the bearer token.
type MCPServer struct {
	// Name is lower-case letters, digits and dashes, and is not "tools-apps", the board's.
	Name string
	// URL is the server's fixed endpoint.
	URL string
	// Token returns the access token to add.
	Token func(ctx context.Context) (string, error)
	// Rejected, when set, is called when the server answers 401, so the next request
	// gets a fresh token.
	Rejected func()
}

// MCPServerName reports whether name can name an MCP server.
func MCPServerName(name string) bool {
	return mcpName.MatchString(name) && name != "tools-apps"
}

var mcpName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,30}$`)

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
	// header names the header auth fills; empty means Authorization.
	header string
	// rejected, when set, hears of a 401 from the upstream.
	rejected func()
	// readOnly refuses everything but GET and HEAD.
	readOnly bool
	// followHosts lists the hosts a redirect from the upstream may be followed to,
	// here at the proxy, since the VM can't reach them.
	followHosts []string
}

// EgressProxy is the only way out of a job's VM (docs/adr-0043): a handful of fixed
// routes to the model providers and the board, with the real credentials added here
// where the VM can't read them. It is not a general proxy: no URL comes from the VM.
type EgressProxy struct {
	routes []route
	rp     *httputil.ReverseProxy
	rt     http.RoundTripper
	log    *slog.Logger
}

type authKey struct{}

// geminiKeyHeader carries Gemini's API key, which it does not take as a bearer.
const geminiKeyHeader = "X-Goog-Api-Key"

// bearer turns a stored key into a route's Authorization.
func bearer(key func() (string, error)) func(context.Context) (string, error) {
	return func(context.Context) (string, error) {
		k, err := key()
		return "Bearer " + k, err
	}
}

// NewEgressProxy builds the proxy for one job.
func NewEgressProxy(cfg EgressConfig) (*EgressProxy, error) {
	p := &EgressProxy{routes: nil, rp: nil, rt: nil, log: cfg.Log}
	if p.log == nil {
		p.log = slog.New(slog.DiscardHandler)
	}
	if err := p.addRoute("anthropic", "/anthropic/", false,
		orDefault(cfg.Anthropic, anthropicURL), nil, nil); err != nil {
		return nil, err
	}
	// Third-party services: the stored key replaces the VM's placeholder.
	for _, svc := range []struct {
		name, prefix, base string
		exact              bool
		key                func() (string, error)
	}{
		{"openrouter", "/openrouter/", orDefault(cfg.OpenRouter, openRouterURL),
			false, cfg.OpenRouterKey},
		{"openai", "/openai/", orDefault(cfg.OpenAI, openAIURL), false, cfg.OpenAIKey},
	} {
		if svc.key == nil {
			continue
		}
		err := p.addRoute(svc.name, svc.prefix, svc.exact, svc.base, bearer(svc.key), nil)
		if err != nil {
			return nil, err
		}
	}
	if err := p.addGeminiRoute(cfg); err != nil {
		return nil, err
	}
	if err := p.addMCPServers(cfg.MCPServers); err != nil {
		return nil, err
	}
	if cfg.MCPToken != nil && cfg.Server != "" {
		token := cfg.MCPToken
		err := p.addRoute("mcp", "/mcp", true, strings.TrimRight(cfg.Server, "/")+"/apps/mcp",
			func(ctx context.Context) (string, error) {
				t, err := token(ctx)
				return "Bearer " + t, err
			}, cfg.MCPRejected)
		if err != nil {
			return nil, err
		}
	}
	// Dependency registries: public, unauthenticated, read-only.
	// The Go proxy answers a module zip with a redirect to its blob store.
	for _, reg := range []struct {
		name, prefix, base string
		follow             []string
	}{
		{"goproxy", "/goproxy/", orDefault(cfg.GoProxy, goProxyURL),
			[]string{"storage.googleapis.com"}},
		{"npm", "/npm/", orDefault(cfg.NPM, npmURL), nil},
		{"rubygems", "/rubygems/", orDefault(cfg.RubyGems, rubyGemsURL), nil},
		{"nixcache", "/nixcache/", orDefault(cfg.NixCache, nixCacheURL), nil},
		{"nixsrc", "/nixsrc/", orDefault(cfg.NixSrc, nixSrcURL), nil},
	} {
		if err := p.addRoute(reg.name, reg.prefix, false, reg.base, nil, nil); err != nil {
			return nil, err
		}
		p.routes[len(p.routes)-1].readOnly = true
		p.routes[len(p.routes)-1].followHosts = reg.follow
	}
	transport := cfg.Transport
	if transport == nil {
		base, ok := http.DefaultTransport.(*http.Transport)
		if !ok {
			return nil, errors.New("no default transport")
		}
		t := base.Clone()
		// The destinations are fixed; the host's HTTPS_PROXY must not reroute the
		// credentials that are added here.
		t.Proxy = nil
		t.ResponseHeaderTimeout = upstreamHeaderMax
		transport = t
	}
	p.rt = transport
	p.rp = &httputil.ReverseProxy{ //nolint:exhaustruct // defaults for the rest
		Rewrite:        p.rewrite,
		Transport:      transport,
		FlushInterval:  -1, // stream events as they come
		ModifyResponse: p.modifyResponse,
		ErrorHandler:   p.upstreamError,
	}
	return p, nil
}

// addGeminiRoute adds /gemini when there is a key; Gemini takes it in a header of its
// own rather than as a bearer.
func (p *EgressProxy) addGeminiRoute(cfg EgressConfig) error {
	if cfg.GeminiKey == nil {
		return nil
	}
	key := cfg.GeminiKey
	err := p.addRoute("gemini", "/gemini/", false, orDefault(cfg.Gemini, geminiURL),
		func(context.Context) (string, error) { return key() }, nil)
	if err != nil {
		return err
	}
	p.routes[len(p.routes)-1].header = geminiKeyHeader
	return nil
}

// addRoute adds a fixed route to base.
func (p *EgressProxy) addRoute(
	name, prefix string, exact bool, base string,
	auth func(context.Context) (string, error), rejected func(),
) error {
	target, err := url.Parse(base)
	if err != nil || target.Scheme == "" || target.Host == "" {
		return fmt.Errorf("egress route %s: bad upstream %q", name, base)
	}
	p.routes = append(p.routes, route{
		name: name, prefix: prefix, exact: exact, target: target, auth: auth,
		rejected: rejected, readOnly: false, header: "", followHosts: nil,
	})
	return nil
}

// addMCPServers adds each server's route, /mcp-<name>.
func (p *EgressProxy) addMCPServers(servers []MCPServer) error {
	for _, srv := range servers {
		if !MCPServerName(srv.Name) || srv.Token == nil {
			return fmt.Errorf("egress route: bad MCP server %q", srv.Name)
		}
		err := p.addRoute("mcp-"+srv.Name, "/mcp-"+srv.Name, true, srv.URL,
			func(ctx context.Context) (string, error) {
				t, err := srv.Token(ctx)
				return "Bearer " + t, err
			}, srv.Rejected)
		if err != nil {
			return err
		}
	}
	return nil
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
	// A route's base is a prefix, so a path that climbs out of it, or that is encoded
	// in a way other than the canonical one, is refused rather than cleaned.
	if r.URL.RawPath != "" || hasDotSegment(r.URL.Path) {
		http.Error(w, "bad path", http.StatusBadRequest)
		return
	}
	rt, rest := p.match(r.URL.Path)
	if rt == nil {
		p.log.Warn("egress refused", "method", r.Method, "path", safePath(r.URL.Path))
		http.Error(w, "not allowed", http.StatusForbidden)
		return
	}
	if rt.readOnly && r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()
	if rt.auth != nil {
		header, err := rt.auth(ctx)
		if err != nil || header == "" || header == "Bearer " {
			// The error never carries a response body or a secret, so it is safe to log.
			p.log.Error("egress credential unavailable", "route", rt.name, "err", err)
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
	// Joined as sent, not cleaned: a trailing slash or a double one is the VM's own.
	out.URL.Path = strings.TrimSuffix(rt.target.Path, "/") + got.rest
	out.URL.RawPath = ""
	for _, h := range []string{
		"Cookie", "Proxy-Authorization", "Proxy-Connection", "Forwarded", "Via",
	} {
		out.Header.Del(h)
	}
	if header, ok := pr.In.Context().Value(authKey{}).(string); ok {
		out.Header.Del("X-Api-Key")
		out.Header.Del(geminiKeyHeader)
		if rt.header != "" {
			out.Header.Del("Authorization")
			out.Header.Set(rt.header, header)
		} else {
			out.Header.Set("Authorization", header)
		}
	}
	p.log.Info("egress", "route", rt.name, "method", out.Method)
}

// modifyResponse keeps an upstream from planting state in the VM, and tells a route
// whose credential was refused.
func (p *EgressProxy) modifyResponse(resp *http.Response) error {
	resp.Header.Del("Set-Cookie")
	got, ok := resp.Request.Context().Value(routeKey{}).(routed)
	if !ok {
		return nil
	}
	if resp.StatusCode == http.StatusUnauthorized && got.route.rejected != nil {
		got.route.rejected()
	}
	return p.followRedirect(resp, got.route)
}

// followRedirect replaces a redirect to one of the route's followHosts with the
// target's own response. Any other redirect passes through to the VM.
func (p *EgressProxy) followRedirect(resp *http.Response, rt *route) error {
	switch resp.StatusCode {
	case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther,
		http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
	default:
		return nil
	}
	loc, err := resp.Location()
	if err != nil || loc.Scheme != "https" ||
		!slices.Contains(rt.followHosts, loc.Hostname()) {
		return nil //nolint:nilerr // an unfollowable redirect passes through
	}
	ctx := resp.Request.Context()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, loc.String(), nil)
	if err != nil {
		return nil //nolint:nilerr // an unfollowable redirect passes through
	}
	next, err := p.rt.RoundTrip(req) //nolint:bodyclose // handed on as resp's body
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	next.Header.Del("Set-Cookie")
	next.Request = resp.Request
	*resp = *next
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

// hasDotSegment reports a "." or ".." path segment.
func hasDotSegment(p string) bool {
	for _, seg := range strings.Split(p, "/") {
		if seg == "." || seg == ".." {
			return true
		}
	}
	return false
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
// services. The returned func stops it and the string is the bound address. It is
// ContainerSandbox's Proxy.
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
