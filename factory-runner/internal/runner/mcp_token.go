package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	// tokenRefreshMargin is how long before expiry a cached token is replaced.
	tokenRefreshMargin = time.Minute
	// tokenFallbackLife is used when the server omits expires_in.
	tokenFallbackLife = 5 * time.Minute
	tokenTimeout      = 15 * time.Second
)

// MCPTokenSource mints the job's board token with the OAuth client_credentials grant
// (docs/adr-0039) and keeps it until shortly before it expires, so the egress proxy
// can add it to each request and the VM never holds it.
type MCPTokenSource struct {
	// ServerURL is the API root, like Config.Server.
	ServerURL    string
	ClientID     string
	ClientSecret string
	HTTP         *http.Client
	// Now is the clock; nil means time.Now.
	Now func() time.Time

	mu       sync.Mutex
	token    string
	expires  time.Time
	endpoint string
}

// Token returns a token with at least tokenRefreshMargin left.
func (s *MCPTokenSource) Token(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.token != "" && s.now().Add(tokenRefreshMargin).Before(s.expires) {
		return s.token, nil
	}
	if s.ClientID == "" || s.ClientSecret == "" {
		return "", errors.New("the job has no board account")
	}
	if s.endpoint == "" {
		endpoint, err := s.discover(ctx)
		if err != nil {
			return "", err
		}
		s.endpoint = endpoint
	}
	token, life, err := s.mint(ctx)
	if err != nil {
		return "", err
	}
	s.token, s.expires = token, s.now().Add(life)
	return token, nil
}

func (s *MCPTokenSource) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *MCPTokenSource) client() *http.Client {
	if s.HTTP != nil {
		return s.HTTP
	}
	return &http.Client{Timeout: tokenTimeout}
}

// discover reads the token endpoint from the authorization server's metadata.
func (s *MCPTokenSource) discover(ctx context.Context) (string, error) {
	var meta struct {
		TokenEndpoint string `json:"token_endpoint"`
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		strings.TrimRight(s.ServerURL, "/")+"/.well-known/oauth-authorization-server",
		nil)
	if err != nil {
		return "", fmt.Errorf("token discovery: %w", err)
	}
	if err = s.do(req, &meta); err != nil {
		return "", fmt.Errorf("token discovery: %w", err)
	}
	if meta.TokenEndpoint == "" {
		return "", errors.New("token discovery: no token endpoint")
	}
	return meta.TokenEndpoint, nil
}

// mint asks the token endpoint for a fresh token.
func (s *MCPTokenSource) mint(ctx context.Context) (string, time.Duration, error) {
	form := url.Values{"grant_type": {"client_credentials"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint,
		strings.NewReader(form.Encode()))
	if err != nil {
		return "", 0, fmt.Errorf("board token: %w", err)
	}
	req.SetBasicAuth(s.ClientID, s.ClientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err = s.do(req, &out); err != nil {
		return "", 0, fmt.Errorf("board token: %w", err)
	}
	if out.AccessToken == "" {
		return "", 0, errors.New("board token: the server sent none")
	}
	life := tokenFallbackLife
	if out.ExpiresIn > 0 {
		life = time.Duration(out.ExpiresIn) * time.Second
	}
	return out.AccessToken, life, nil
}

// do sends req and decodes a 200 JSON answer into out. The body of a failure is
// never returned: it could echo a credential.
func (s *MCPTokenSource) do(req *http.Request, out any) error {
	resp, err := s.client().Do(req)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseBytes))
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	if err = json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).
		Decode(out); err != nil {
		return fmt.Errorf("answer: %w", err)
	}
	return nil
}
