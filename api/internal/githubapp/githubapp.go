// Package githubapp mints short-lived GitHub App installation tokens, so the
// factory can push a job's branch and open its pull request without any
// long-lived GitHub credential on the runner (adr-0045).
package githubapp

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultBaseURL = "https://api.github.com"
	requestTimeout = 15 * time.Second
	// jwtSkew backdates the JWT's issue time: GitHub rejects one from its future.
	jwtSkew = time.Minute
	// jwtLifetime is under GitHub's ten-minute cap.
	jwtLifetime = 9 * time.Minute
	maxBody     = 1 << 20
)

// ErrNotConfigured is returned when the App's id or key is missing or invalid.
var ErrNotConfigured = errors.New("github app is not configured")

// ErrNotInstalled is returned when the App isn't installed on the repository.
var ErrNotInstalled = errors.New("github app is not installed on the repository")

// namePattern is what a GitHub owner or repository name may contain; anything
// else is refused before it reaches a URL path.
var namePattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,100}$`)

// validName is a GitHub owner or repository name, and not a path step.
func validName(name string) bool {
	return name != "." && name != ".." && namePattern.MatchString(name)
}

// Token is an installation access token and when GitHub stops honouring it.
type Token struct {
	Value     string
	ExpiresAt time.Time
}

// Client mints installation tokens for one GitHub App.
type Client struct {
	appID   string
	key     *rsa.PrivateKey
	baseURL string
	http    *http.Client
	now     func() time.Time

	mu            sync.Mutex
	installations map[string]int64
	// readTokens are the read-only tokens of pullrequests.go, by owner/repo.
	readTokens map[string]Token
}

// Option adjusts a Client; tests point it at a fake GitHub.
type Option func(*Client)

// WithBaseURL replaces https://api.github.com.
func WithBaseURL(url string) Option { return func(c *Client) { c.baseURL = url } }

// WithHTTPClient replaces the default client.
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.http = h } }

// WithClock replaces time.Now.
func WithClock(now func() time.Time) Option { return func(c *Client) { c.now = now } }

// New parses the App's private key (PKCS#1 or PKCS#8 PEM). An empty id or key is
// ErrNotConfigured.
func New(appID, privateKeyPEM string, opts ...Option) (*Client, error) {
	if appID == "" || privateKeyPEM == "" {
		return nil, ErrNotConfigured
	}
	key, err := parseKey(privateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNotConfigured, err)
	}
	c := &Client{
		appID: appID, key: key, baseURL: defaultBaseURL,
		http: &http.Client{Timeout: requestTimeout}, now: time.Now,
		mu: sync.Mutex{}, installations: map[string]int64{},
		readTokens: map[string]Token{},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

func parseKey(pemText string) (*rsa.PrivateKey, error) {
	// An env file may carry the key's newlines as the two characters \n.
	pemText = strings.ReplaceAll(pemText, `\n`, "\n")
	block, _ := pem.Decode([]byte(pemText))
	if block == nil {
		return nil, errors.New("not a PEM block")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, errors.New("not an RSA private key")
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("not an RSA private key")
	}
	return key, nil
}

// appJWT is the App's own credential, valid minutes, used only to ask for an
// installation token.
func (c *Client) appJWT() (string, error) {
	now := c.now()
	header, err := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT"})
	if err != nil {
		return "", err
	}
	claims, err := json.Marshal(map[string]any{
		"iat": now.Add(-jwtSkew).Unix(),
		"exp": now.Add(jwtLifetime).Unix(),
		"iss": c.appID,
	})
	if err != nil {
		return "", err
	}
	enc := base64.RawURLEncoding
	signing := enc.EncodeToString(header) + "." + enc.EncodeToString(claims)
	sum := sha256.Sum256([]byte(signing))
	sig, err := rsa.SignPKCS1v15(rand.Reader, c.key, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}
	return signing + "." + enc.EncodeToString(sig), nil
}

func (c *Client) do(
	ctx context.Context,
	method, path, jwt string,
	body any,
	out any,
) (int, error) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return 0, err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return resp.StatusCode, err
	}
	if resp.StatusCode >= http.StatusMultipleChoices {
		// The body is left out: it is GitHub's text, and none of it is ours to log.
		return resp.StatusCode, fmt.Errorf("github: %s %s: status %d",
			method, path, resp.StatusCode)
	}
	if out != nil {
		if err = json.Unmarshal(data, out); err != nil {
			return resp.StatusCode, fmt.Errorf("github: %s %s: bad response", method, path)
		}
	}
	return resp.StatusCode, nil
}

// installationID is the App's installation on the repository, remembered.
func (c *Client) installationID(
	ctx context.Context,
	jwt, owner, repo string,
) (int64, error) {
	key := owner + "/" + repo
	c.mu.Lock()
	id, ok := c.installations[key]
	c.mu.Unlock()
	if ok {
		return id, nil
	}
	var found struct {
		ID int64 `json:"id"`
	}
	status, err := c.do(ctx, http.MethodGet, "/repos/"+owner+"/"+repo+"/installation",
		jwt, nil, &found)
	if status == http.StatusNotFound {
		return 0, ErrNotInstalled
	}
	if err != nil {
		return 0, err
	}
	if found.ID == 0 {
		return 0, errors.New("github: installation without an id")
	}
	c.mu.Lock()
	c.installations[key] = found.ID
	c.mu.Unlock()
	return found.ID, nil
}

// InstallationToken mints a token for owner/repo alone, with just what a job
// needs: contents write (the push) and pull requests write (the PR). It expires
// within the hour and can't touch workflows, issues or any other repository.
func (c *Client) InstallationToken(
	ctx context.Context,
	owner, repo string,
) (Token, error) {
	return c.mint(ctx, owner, repo,
		map[string]string{"contents": "write", "pull_requests": "write"})
}

// mint asks for an installation token limited to owner/repo and the permissions.
func (c *Client) mint(
	ctx context.Context,
	owner, repo string,
	permissions map[string]string,
) (Token, error) {
	if !validName(owner) || !validName(repo) {
		return Token{}, errors.New("github: invalid repository name")
	}
	jwt, err := c.appJWT()
	if err != nil {
		return Token{}, err
	}
	id, err := c.installationID(ctx, jwt, owner, repo)
	if err != nil {
		return Token{}, err
	}
	var minted struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	status, err := c.do(ctx, http.MethodPost,
		"/app/installations/"+strconv.FormatInt(id, 10)+"/access_tokens", jwt,
		map[string]any{"repositories": []string{repo}, "permissions": permissions},
		&minted)
	if err != nil {
		// Whatever went wrong, don't keep the id: after a reinstall GitHub may
		// answer for the old one with 404, 403 or 422, and a stale id must not
		// outlive the next call.
		c.mu.Lock()
		delete(c.installations, owner+"/"+repo)
		c.mu.Unlock()
		if status == http.StatusNotFound {
			return Token{}, ErrNotInstalled
		}
		return Token{}, err
	}
	if minted.Token == "" {
		return Token{}, errors.New("github: empty installation token")
	}
	return Token{Value: minted.Token, ExpiresAt: minted.ExpiresAt}, nil
}
