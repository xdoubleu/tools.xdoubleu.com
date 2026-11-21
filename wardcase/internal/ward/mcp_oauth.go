package ward

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

// The MCP authorization flow (spec 2025-06-18) runs on the host: the server's
// Protected Resource Metadata (RFC 9728) names its authorization server, whose
// metadata (RFC 8414) gives the endpoints; the client registers itself (RFC 7591) and
// signs in with the authorization code and PKCE through a loopback redirect, naming
// the server as the resource (RFC 8707).

const (
	// oauthTimeout bounds each discovery, registration and token request.
	oauthTimeout = 30 * time.Second
	// loginTimeout bounds the wait for the browser sign-in.
	loginTimeout = 5 * time.Minute
	// maxOAuthBody bounds any metadata or token response read.
	maxOAuthBody = 1 << 20
	clientName   = "wardcase"
)

// MCPCreds is what a signed-in server keeps in the keychain: the registration and
// the refresh token. Access tokens are never stored.
type MCPCreds struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret,omitempty"`
	TokenURL     string `json:"token_url"`
	Resource     string `json:"resource"`
	RefreshToken string `json:"refresh_token"`
}

// authServer is the part of RFC 8414 metadata the flow uses.
type authServer struct {
	Issuer                string   `json:"issuer"`
	AuthorizationEndpoint string   `json:"authorization_endpoint"`
	TokenEndpoint         string   `json:"token_endpoint"`
	RegistrationEndpoint  string   `json:"registration_endpoint"`
	CodeChallengeMethods  []string `json:"code_challenge_methods_supported"`
}

// client is a registered OAuth client.
type client struct{ id, secret string }

type resourceMeta struct {
	AuthorizationServers []string `json:"authorization_servers"`
	ScopesSupported      []string `json:"scopes_supported"`
}

var metadataURL = regexp.MustCompile(`resource_metadata="([^"]+)"`)

func getJSON(ctx context.Context, hc *http.Client, target string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return fmt.Errorf("get %s: %w", target, err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("get %s: %w", target, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("get %s: status %d", target, resp.StatusCode)
	}
	err = json.NewDecoder(io.LimitReader(resp.Body, maxOAuthBody)).Decode(out)
	if err != nil {
		return fmt.Errorf("get %s: %w", target, err)
	}
	return nil
}

// wellKnown is the RFC 8615 location for a resource or issuer, the well-known name
// inserted between the host and the path.
func wellKnown(base *url.URL, name string) string {
	u := *base
	u.Path = "/.well-known/" + name + strings.TrimSuffix(base.Path, "/")
	u.RawQuery, u.Fragment = "", ""
	return u.String()
}

// protectedResource reads the server's Protected Resource Metadata, from the URL its
// 401 names, else from the well-known location.
func protectedResource(
	ctx context.Context, hc *http.Client, server *url.URL,
) (resourceMeta, error) {
	var meta resourceMeta
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.String(), nil)
	if err != nil {
		return meta, fmt.Errorf("probe %s: %w", server, err)
	}
	req.Header.Set("Accept", "application/json, text/event-stream")
	candidates := []string{wellKnown(server, "oauth-protected-resource")}
	if resp, doErr := hc.Do(req); doErr == nil {
		_ = resp.Body.Close()
		challenge := resp.Header.Get("WWW-Authenticate")
		if m := metadataURL.FindStringSubmatch(challenge); m != nil {
			candidates = append([]string{m[1]}, candidates...)
		}
	}
	for _, c := range candidates {
		err = getJSON(ctx, hc, c, &meta)
		if err == nil && len(meta.AuthorizationServers) > 0 {
			return meta, nil
		}
	}
	return meta, fmt.Errorf("%s does not describe its authorization server "+
		"(RFC 9728): %w", server, errors.Join(err))
}

// authorizationServer reads the issuer's metadata, trying RFC 8414's location then
// OpenID Connect's, and checks it is usable.
func authorizationServer(
	ctx context.Context, hc *http.Client, issuer string,
) (authServer, error) {
	var meta authServer
	base, err := url.Parse(issuer)
	if err != nil || !secureURL(base) {
		return meta, fmt.Errorf("authorization server %q must be https", issuer)
	}
	oidc := *base
	oidc.Path = strings.TrimSuffix(base.Path, "/") + "/.well-known/openid-configuration"
	rfc8414 := wellKnown(base, "oauth-authorization-server")
	for _, c := range []string{rfc8414, oidc.String()} {
		if err = getJSON(ctx, hc, c, &meta); err == nil {
			break
		}
	}
	if err != nil {
		return meta, fmt.Errorf("authorization server metadata: %w", err)
	}
	for _, e := range []string{
		meta.AuthorizationEndpoint, meta.TokenEndpoint, meta.RegistrationEndpoint,
	} {
		if u, parseErr := url.Parse(e); parseErr != nil || !secureURL(u) {
			return meta, fmt.Errorf("%s: endpoint %q is missing or not https "+
				"(dynamic client registration is required)", issuer, e)
		}
	}
	if len(meta.CodeChallengeMethods) > 0 && !contains(meta.CodeChallengeMethods, "S256") {
		return meta, fmt.Errorf("%s does not support PKCE S256", issuer)
	}
	return meta, nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func secureURL(u *url.URL) bool {
	return u != nil && u.Host != "" &&
		(u.Scheme == "https" || (u.Scheme == "http" && isLoopback(u.Hostname())))
}

// register makes this client known to the authorization server (RFC 7591) as a
// public client with the loopback redirect.
func register(
	ctx context.Context, hc *http.Client, endpoint, redirect string,
) (client, error) {
	var none client
	body, err := json.Marshal(map[string]any{
		"client_name": clientName, "redirect_uris": []string{redirect},
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
		"token_endpoint_auth_method": "none",
	})
	if err != nil {
		return none, fmt.Errorf("register: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint,
		bytes.NewReader(body))
	if err != nil {
		return none, fmt.Errorf("register: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		return none, fmt.Errorf("register: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return none, fmt.Errorf("register: status %d", resp.StatusCode)
	}
	var out struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	}
	err = json.NewDecoder(io.LimitReader(resp.Body, maxOAuthBody)).Decode(&out)
	if err != nil || out.ClientID == "" {
		return none, errors.New("register: the server returned no client_id")
	}
	return client{id: out.ClientID, secret: out.ClientSecret}, nil
}

func (c MCPCreds) config() *oauth2.Config {
	return &oauth2.Config{
		ClientID: c.ClientID, ClientSecret: c.ClientSecret, RedirectURL: "",
		Endpoint: oauth2.Endpoint{
			AuthURL: "", DeviceAuthURL: "", TokenURL: c.TokenURL,
			AuthStyle: oauth2.AuthStyleInParams,
		},
		Scopes: nil,
	}
}

// MCPLogin signs a server in on the host.
type MCPLogin struct {
	// HTTP reaches the server and its authorization server; nil is a default client.
	HTTP *http.Client
	// Open shows the sign-in page to the user; it may be nil, as the URL is printed too.
	Open func(url string) error
	// Out gets the sign-in URL.
	Out io.Writer
}

// Login runs the flow for one configured server and returns what to keep.
func (l MCPLogin) Login(ctx context.Context, server MCPConfig) (MCPCreds, error) {
	hc := l.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: oauthTimeout}
	}
	var none MCPCreds
	resource, err := url.Parse(server.URL)
	if err != nil {
		return none, fmt.Errorf("mcp %s: %w", server.Name, err)
	}
	prm, err := protectedResource(ctx, hc, resource)
	if err != nil {
		return none, err
	}
	as, err := authorizationServer(ctx, hc, prm.AuthorizationServers[0])
	if err != nil {
		return none, err
	}
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		return none, fmt.Errorf("loopback redirect: %w", err)
	}
	defer ln.Close()
	redirect := "http://" + ln.Addr().String() + "/callback"
	reg, err := register(ctx, hc, as.RegistrationEndpoint, redirect)
	if err != nil {
		return none, err
	}
	creds := MCPCreds{
		ClientID: reg.id, ClientSecret: reg.secret, TokenURL: as.TokenEndpoint,
		Resource: server.URL, RefreshToken: "",
	}
	cfg := creds.config()
	cfg.Endpoint.AuthURL, cfg.RedirectURL, cfg.Scopes = as.AuthorizationEndpoint, redirect,
		prm.ScopesSupported
	verifier, state := oauth2.GenerateVerifier(), oauth2.GenerateVerifier()
	authURL := cfg.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier),
		oauth2.SetAuthURLParam("resource", server.URL))
	fmt.Fprintf(l.Out, "Sign in to %s in your browser:\n%s\n", server.Name, authURL)
	if l.Open != nil {
		_ = l.Open(authURL)
	}
	code, err := awaitCode(ctx, ln, state)
	if err != nil {
		return none, err
	}
	tok, err := cfg.Exchange(context.WithValue(ctx, oauth2.HTTPClient, hc), code,
		oauth2.VerifierOption(verifier), oauth2.SetAuthURLParam("resource", server.URL))
	if err != nil {
		return none, fmt.Errorf("sign-in token exchange: %w", err)
	}
	if tok.RefreshToken == "" {
		return none, errors.New("the server issued no refresh token, so wardcase " +
			"could not stay signed in")
	}
	creds.RefreshToken = tok.RefreshToken
	return creds, nil
}

// awaitCode serves the loopback redirect until it carries the authorization code
// for state.
func awaitCode(ctx context.Context, ln net.Listener, state string) (string, error) {
	type result struct {
		code string
		err  error
	}
	got := make(chan result, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		var res result
		switch {
		case q.Get("state") != state:
			http.Error(w, "unexpected state", http.StatusBadRequest)
			return
		case q.Get("error") != "":
			res.err = fmt.Errorf("the server refused the sign-in: %s", q.Get("error"))
		case q.Get("code") == "":
			http.Error(w, "no code", http.StatusBadRequest)
			return
		default:
			res.code = q.Get("code")
		}
		_, _ = io.WriteString(w, "Signed in. You can close this tab.")
		select {
		case got <- res:
		default:
		}
	})
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: oauthTimeout}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()
	ctx, cancel := context.WithTimeout(ctx, loginTimeout)
	defer cancel()
	select {
	case res := <-got:
		return res.code, res.err
	case <-ctx.Done():
		return "", fmt.Errorf("waiting for the sign-in: %w", ctx.Err())
	}
}
