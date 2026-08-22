package oauth2as

import (
	"slices"
	"strings"

	"github.com/ory/fosite"
)

// OfflineAccessScope is the scope fosite requires among a request's *granted*
// scopes before the token endpoint will issue a refresh token. Every client
// this server registers carries it (see RegisterClient), and it is the only
// scope this server supports.
const OfflineAccessScope = "offline_access"

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
