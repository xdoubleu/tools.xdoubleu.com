package oauth2as

import (
	"slices"
	"strings"

	"github.com/ory/fosite"
)

// The scopes this authorization server understands.
//
//   - OfflineAccessScope is what fosite requires among a request's *granted*
//     scopes before the token endpoint will issue a refresh token. Every
//     dynamically-registered (MCP) client carries it (see RegisterClient).
//   - OpenIDScope / ProfileScope / EmailScope drive OIDC ID-token issuance
//     and its claims (issue #1469); only clients that register them — today
//     just the static Grafana SSO client — may request them.
const (
	OpenIDScope        = "openid"
	ProfileScope       = "profile"
	EmailScope         = "email"
	OfflineAccessScope = "offline_access"
)

// SupportedScopes is every scope this server advertises in its metadata
// documents and is willing to grant to a client that has it registered.
var SupportedScopes = []string{ //nolint:gochecknoglobals // read-only scope list
	OpenIDScope, ProfileScope, EmailScope, OfflineAccessScope,
}

// grantOfflineAccess grants offline_access on top of whatever the client
// actually asked for, provided the registered client is allowed to hold it.
// MCP clients routinely send no scope parameter at all; without this they
// would get an access token with no refresh token and be forced through an
// interactive re-authentication every time it expired. Granting a scope the
// client didn't request is safe here because the client's scope list is
// server-controlled — RegisterClient hardcodes it and RegisterHandler ignores
// any client-supplied scope.
func grantOfflineAccess(ar fosite.AuthorizeRequester) {
	client := ar.GetClient()
	if client == nil || !slices.Contains(client.GetScopes(), OfflineAccessScope) {
		return
	}
	ar.GrantScope(OfflineAccessScope)
}

// effectiveScope is what the consent screen must display: the scopes the
// client requested plus the offline_access grantOfflineAccess will add, so
// the screen never understates what approving actually authorizes.
func effectiveScope(requested string, clientScopes []string) string {
	scopes := strings.Fields(requested)
	if slices.Contains(clientScopes, OfflineAccessScope) &&
		!slices.Contains(scopes, OfflineAccessScope) {
		scopes = append(scopes, OfflineAccessScope)
	}
	return strings.Join(scopes, " ")
}
