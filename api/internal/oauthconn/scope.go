package oauthconn

import (
	"strings"

	"tools.xdoubleu.com/internal/models"
)

// ScopesAreStale reports whether conn lacks a required scope and must be
// re-authorized. It compares the *requested* scope: providers normalize the
// granted echo (GitHub reduces `repo security_events` to `repo`). Rows without
// a requested scope fall back to the granted check.
func ScopesAreStale(conn *models.OAuthConnection, required []string) bool {
	if conn == nil {
		return false
	}
	if conn.RequestedScope != "" {
		return !HasScopes(conn.RequestedScope, required)
	}
	return !HasScopes(conn.GrantedScope, required)
}

// HasScopes reports whether granted (space- or comma-separated; GitHub uses
// commas) covers required. Empty granted means unknown and passes.
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
