package oauthconn

import (
	"strings"

	"tools.xdoubleu.com/internal/models"
)

// ScopesAreStale reports whether conn was authorized with less than required
// covers — i.e. it predates a scope the provider's oauth2.Config now asks for
// and must be re-authorized.
//
// It compares what was *requested* at connect time, never what the provider
// echoed back: a provider is free to normalize the granted scope string, and
// GitHub does, collapsing `repo security_events` to just `repo` because the
// former subsumes the latter. Judging coverage by that echo marked every
// freshly-connected GitHub account stale forever, so the admin UI kept
// offering "Connect" no matter how many times an admin completed the flow.
//
// Connections stored before the requested scope was recorded have no such
// value; those fall back to the granted-scope check, and heal on the next
// reconnect.
func ScopesAreStale(conn *models.OAuthConnection, required []string) bool {
	if conn == nil {
		return false
	}
	if conn.RequestedScope != "" {
		return !HasScopes(conn.RequestedScope, required)
	}
	return !HasScopes(conn.GrantedScope, required)
}

// HasScopes reports whether granted (the `scope` string a provider returned
// with a token, space- or comma-separated — GitHub uses commas, most other
// RFC 6749-compliant providers use spaces) covers every scope in required.
// An empty granted string is treated as "unknown" (older connections predate
// storing it) rather than "none", so it always passes — only a connection
// with a recorded, incomplete scope list is flagged stale.
func HasScopes(granted string, required []string) bool {
	if granted == "" {
		return true
	}

	have := make(map[string]bool)
	for _, s := range strings.FieldsFunc(granted, func(r rune) bool {
		return r == ' ' || r == ','
	}) {
		have[s] = true
	}

	for _, s := range required {
		if !have[s] {
			return false
		}
	}
	return true
}
