package oauthconn

import "strings"

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
