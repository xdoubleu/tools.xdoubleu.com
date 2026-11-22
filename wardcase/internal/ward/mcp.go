package ward

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"

	"tools.xdoubleu.com/wardcase/vm"
)

// MCP servers are device configuration (config.toml's [[mcp]]) and signed in on the
// host (mcp_oauth.go). The keychain keeps the registration and refresh token; the
// egress proxy adds the access token, so no token enters the VM (adr-0055).

// mcpAccount is the keychain account of a server's sign-in.
func mcpAccount(name string) string { return "mcp-" + name }

// SaveMCP keeps a server's sign-in in the keychain.
func SaveMCP(store Secrets, name string, creds MCPCreds) error {
	data, err := json.Marshal(creds) //nolint:gosec // for the keychain
	if err != nil {
		return fmt.Errorf("mcp %s: %w", name, err)
	}
	return store.Set(mcpAccount(name), string(data))
}

// LoginMCP signs the named configured server in and keeps the result.
func LoginMCP(
	ctx context.Context, cfg Config, store Secrets, l MCPLogin, name string,
) error {
	server, ok := cfg.MCPServer(name)
	if !ok {
		return fmt.Errorf("no [[mcp]] named %q in the config", name)
	}
	creds, err := l.Login(ctx, server)
	if err != nil {
		return err
	}
	return SaveMCP(store, name, creds)
}

// LogoutMCP removes a server's stored sign-in.
func LogoutMCP(store Secrets, name string) error {
	return store.Delete(mcpAccount(name))
}

func loadMCP(store Secrets, name string) (MCPCreds, bool, error) {
	var none MCPCreds
	raw, err := store.Get(mcpAccount(name))
	if errors.Is(err, ErrNoSecret) {
		return none, false, nil
	}
	if err != nil {
		return none, false, err
	}
	var creds MCPCreds
	if err = json.Unmarshal([]byte(raw), &creds); err != nil || creds.RefreshToken == "" {
		return none, false, nil //nolint:nilerr // unreadable means not signed in
	}
	return creds, true, nil
}

// MCPSignedIn reports whether the named server has a stored sign-in.
func MCPSignedIn(store Secrets, name string) (bool, error) {
	_, ok, err := loadMCP(store, name)
	return ok, err
}

// mcpToken hands out a server's access token, refreshing it when it expires or the
// server rejected it, and keeping a rotated refresh token.
type mcpToken struct {
	hc    *http.Client
	name  string
	store Secrets

	mu    sync.Mutex
	creds MCPCreds
	tok   *oauth2.Token
}

// Token returns the current access token.
func (m *mcpToken) Token(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.tok.Valid() {
		return m.tok.AccessToken, nil
	}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, m.hc)
	src := m.creds.config().TokenSource(ctx, &oauth2.Token{
		AccessToken: "", TokenType: "", RefreshToken: m.creds.RefreshToken,
		Expiry: time.Time{}, ExpiresIn: 0,
	})
	tok, err := src.Token()
	if err != nil {
		return "", fmt.Errorf("refresh the %s sign-in (wardcase mcp login %s): %w",
			m.name, m.name, err)
	}
	if tok.RefreshToken != "" && tok.RefreshToken != m.creds.RefreshToken {
		m.creds.RefreshToken = tok.RefreshToken
		if err = SaveMCP(m.store, m.name, m.creds); err != nil {
			return "", err
		}
	}
	m.tok = tok
	return tok.AccessToken, nil
}

// Reject drops the access token the server refused, so the next request refreshes.
func (m *mcpToken) Reject() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tok = nil
}

// MCPServers is the proxy routes for the given servers that are signed in, and the
// names of the ones that are not. Headless runs get only servers with jobs set.
func MCPServers(
	configured []MCPConfig, store Secrets, headless bool,
) ([]vm.MCPServer, []string, error) {
	var servers []vm.MCPServer
	var missing []string
	hc := &http.Client{Timeout: oauthTimeout}
	for _, server := range configured {
		if headless && !server.Jobs {
			continue
		}
		creds, ok, loadErr := loadMCP(store, server.Name)
		if loadErr != nil {
			return nil, nil, fmt.Errorf("mcp %s: %w", server.Name, loadErr)
		}
		if !ok {
			missing = append(missing, server.Name)
			continue
		}
		tok := &mcpToken{
			hc: hc, name: server.Name, store: store, creds: creds,
			mu: sync.Mutex{}, tok: nil,
		}
		servers = append(servers, vm.MCPServer{
			Name: server.Name, URL: server.URL, Token: tok.Token, Rejected: tok.Reject,
		})
	}
	return servers, missing, nil
}

// addMCP puts the configured, signed-in servers on the proxy and tells the VM their
// names; the ones not signed in are named on Notes.
func (r *Runner) addMCP(
	egress *vm.EgressConfig, env map[string]string, headless bool, checkout string,
) error {
	repo, err := LoadRepoConfig(checkout)
	if err != nil {
		return err
	}
	allowed, unknown := repo.narrow(r.Config.MCP)
	for _, name := range unknown {
		fmt.Fprintf(r.Notes, "wardcase: the repo lists MCP server %s, which the device "+
			"does not configure\n", name)
	}
	servers, missing, err := MCPServers(allowed, r.Secrets, headless)
	if err != nil {
		return err
	}
	egress.MCPServers = servers
	names := make([]string, 0, len(servers))
	for _, s := range servers {
		names = append(names, s.Name)
	}
	slices.Sort(names)
	if len(names) > 0 {
		env[vm.MCPServersEnv] = strings.Join(names, " ")
	}
	for _, name := range missing {
		fmt.Fprintf(r.Notes,
			"wardcase: MCP server %s is not signed in (wardcase mcp login %s)\n", name, name)
	}
	return nil
}
