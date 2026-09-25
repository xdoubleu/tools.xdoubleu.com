package oauth2as

import (
	"time"

	"github.com/ory/fosite"
	"github.com/ory/fosite/handler/openid"
	"github.com/ory/fosite/token/jwt"
)

// ResolvedUser is the web-session identity ID-token claims are built from.
// Email/DisplayName need their scope; IsAdmin gates the role claim.
type ResolvedUser struct {
	ID          string
	Email       string
	DisplayName string
	IsAdmin     bool
}

// newAuthorizeSession builds the session fosite persists at authorize time.
// Subject is always set (MCP needs it); ID-token claims only with openid. kid
// lets relying parties pick the key from /oauth2/jwks.
func newAuthorizeSession(
	user ResolvedUser, kid string, grantedScopes fosite.Arguments,
) *openid.DefaultSession {
	now := time.Now().UTC()
	//nolint:exhaustruct //ExpiresAt is populated by fosite as tokens are issued
	sess := &openid.DefaultSession{
		Subject:  user.ID,
		Username: user.Email,
		Headers:  &jwt.Headers{Extra: map[string]any{"kid": kid}},
		//nolint:exhaustruct //remaining registered claims are set by fosite's OIDC strategy
		Claims: &jwt.IDTokenClaims{
			Subject:     user.ID,
			IssuedAt:    now,
			RequestedAt: now,
			AuthTime:    now,
		},
	}

	if grantedScopes.Has(OpenIDScope) {
		addProfileClaims(sess.Claims, user, grantedScopes)
	}
	return sess
}

// addProfileClaims adds OIDC claims per granted scope. role="Admin" is emitted
// only for admins: Grafana (role_attribute_strict) is the sole relying party
// and is admin-only, so non-admins are refused there.
func addProfileClaims(
	claims *jwt.IDTokenClaims, user ResolvedUser, grantedScopes fosite.Arguments,
) {
	if user.IsAdmin {
		claims.Add("role", "Admin")
	}

	if grantedScopes.Has(EmailScope) && user.Email != "" {
		claims.Add("email", user.Email)
		// Accounts are admin-provisioned, so email is always verified.
		claims.Add("email_verified", true)
	}

	if grantedScopes.Has(ProfileScope) {
		name := user.DisplayName
		if name == "" {
			name = user.Email
		}
		claims.Add("name", name)
		claims.Add("preferred_username", user.Email)
	}
}
