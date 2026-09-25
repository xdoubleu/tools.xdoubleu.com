package oauth2as

import (
	"slices"
	"strings"

	"github.com/ory/fosite"
)

// Scopes this server understands. fosite issues a refresh token only when
// OfflineAccessScope is granted (every MCP client has it). The OIDC scopes are
// only for clients that register them (Grafana).
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

// grantOfflineAccess grants offline_access even if not requested (MCP clients
// often send no scope), when the client may hold it. Safe because client scope
// lists are server-controlled.
func grantOfflineAccess(ar fosite.AuthorizeRequester) {
	client := ar.GetClient()
	if client == nil || !slices.Contains(client.GetScopes(), OfflineAccessScope) {
		return
	}
	ar.GrantScope(OfflineAccessScope)
}

// effectiveScope is what consent must display: requested scopes plus the
// offline_access grantOfflineAccess adds.
func effectiveScope(requested string, clientScopes []string) string {
	scopes := strings.Fields(requested)
	if slices.Contains(clientScopes, OfflineAccessScope) &&
		!slices.Contains(scopes, OfflineAccessScope) {
		scopes = append(scopes, OfflineAccessScope)
	}
	return strings.Join(scopes, " ")
}
