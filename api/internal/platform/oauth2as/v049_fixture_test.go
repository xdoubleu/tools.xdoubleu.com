package oauth2as_test

import (
	"encoding/json"
	"time"
)

// v049FixturePath holds token strings and the auth.oauth2_* rows behind them,
// as fosite v0.49.0 wrote them.
const v049FixturePath = "testdata/fosite-v0.49.0.json"

// Secrets the fixture was generated with; test-only values.
const (
	v049HMACSecret       = "v049-fixture-hmac-secret-32bytes"
	v049OIDCClientID     = "v049-fixture-oidc"
	v049OIDCClientSecret = "v049-fixture-oidc-client-secret"
	v049RoutinesSecret   = "v049-fixture-routines-client-secret"
	v049OIDCRedirectURI  = "https://example.com/v049/callback"
	v049OIDCScope        = "openid profile email offline_access"
)

// v049Tokens are the strings a client held when the fixture was captured.
type v049Tokens struct {
	ClientID     string `json:"client_id"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	// StaleRefreshToken was rotated out by RefreshToken's grant.
	StaleRefreshToken string `json:"stale_refresh_token,omitempty"`
}

type v049Code struct {
	ClientID    string `json:"client_id"`
	Code        string `json:"code"`
	Verifier    string `json:"verifier"`
	RedirectURI string `json:"redirect_uri"`
}

type v049Fixture struct {
	CapturedAt time.Time `json:"captured_at"`
	// MCP: a dynamically registered client sending no scope, so the stored
	// ID-token claims carry "ext": null.
	MCP v049Tokens `json:"mcp"`
	// Rotated: an MCP chain refreshed once.
	Rotated v049Tokens `json:"rotated"`
	// OIDC: a confidential client granted v049OIDCScope.
	OIDC v049Tokens `json:"oidc"`
	// PendingCode: an unredeemed openid authorization code with PKCE.
	PendingCode       v049Code   `json:"pending_code"`
	ClientCredentials v049Tokens `json:"client_credentials"`
	// Rows maps each auth.oauth2_* table to its rows as to_jsonb(row).
	Rows map[string][]json.RawMessage `json:"rows"`
}

// v049Tables lists the fixture tables in foreign-key order.
//
//nolint:gochecknoglobals //read-only list
var v049Tables = []string{
	"oauth2_clients",
	"oauth2_auth_codes",
	"oauth2_pkce_requests",
	"oauth2_oidc_sessions",
	"oauth2_access_tokens",
	"oauth2_refresh_tokens",
}
