package oauth2as

import (
	"time"

	"github.com/ory/fosite"
	"github.com/ory/fosite/handler/openid"
	"github.com/ory/fosite/token/jwt"
)

// ResolvedUser is the identity the composition root resolves from the web
// session cookie during the authorize flow — everything the ID token's claims
// can be built from. Email/DisplayName are only surfaced when the matching
// scope (email / profile) was granted; IsAdmin always drives the role claim.
type ResolvedUser struct {
	ID          string
	Email       string
	DisplayName string
	IsAdmin     bool
}

// newAuthorizeSession builds the session fosite persists at authorize time.
// Subject is always set — the MCP flow depends on it via ResolveAccessToken —
// while the ID-token claims are only populated when the openid scope was
// granted, which is also the only case fosite mints an ID token for. kid is
// stamped into the JWT header so a relying party can select the right key
// from /oauth2/jwks.
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

// addProfileClaims layers the OIDC identity claims onto an ID token per the
// scopes actually granted. The role claim is emitted unconditionally: Grafana
// maps it onto its Admin/Viewer roles via role_attribute_path, and a login
// with no resolvable role is worse than no login.
func addProfileClaims(
	claims *jwt.IDTokenClaims, user ResolvedUser, grantedScopes fosite.Arguments,
) {
	role := "Viewer"
	if user.IsAdmin {
		role = "Admin"
	}
	claims.Add("role", role)

	if grantedScopes.Has(EmailScope) && user.Email != "" {
		claims.Add("email", user.Email)
		// First-party accounts are admin-provisioned; there is no
		// unverified-email state to represent (ADR-0005).
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
