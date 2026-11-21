package oauth2as

import (
	"slices"
	"strings"

	fosite "authelia.com/provider/oauth2"
)

// Scopes this server understands. fosite issues a refresh token / ID token
// only when OfflineAccessScope / OpenIDScope is granted; every dynamically
// registered (MCP) client carries both. Profile/Email add claims to the ID
// token and are only for clients that register them (Grafana).
const (
	OpenIDScope        = "openid"
	ProfileScope       = "profile"
	EmailScope         = "email"
	OfflineAccessScope = "offline_access"
)

// SupportedScopes is every scope advertised in metadata.
var SupportedScopes = []string{ //nolint:gochecknoglobals // read-only scope list
	OpenIDScope, ProfileScope, EmailScope, OfflineAccessScope,
}

// GrantOfflineAccess grants offline_access even if not requested (MCP clients
// often send no scope), when the client may hold it. Safe because client scope
// lists are server-controlled.
func GrantOfflineAccess(ar fosite.AuthorizeRequester) {
	client := ar.GetClient()
	if client == nil || !slices.Contains(client.GetScopes(), OfflineAccessScope) {
		return
	}
	ar.GrantScope(OfflineAccessScope)
}

// EffectiveScope is what consent must display: requested scopes plus the
// offline_access GrantOfflineAccess adds.
func EffectiveScope(requested string, clientScopes []string) string {
	scopes := strings.Fields(requested)
	if slices.Contains(clientScopes, OfflineAccessScope) &&
		!slices.Contains(scopes, OfflineAccessScope) {
		scopes = append(scopes, OfflineAccessScope)
	}
	return strings.Join(scopes, " ")
}
