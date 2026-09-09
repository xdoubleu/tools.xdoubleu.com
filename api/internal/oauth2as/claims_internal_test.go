package oauth2as

import (
	"testing"

	"github.com/ory/fosite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAuthorizeSession_NoOpenIDScope(t *testing.T) {
	sess := newAuthorizeSession(
		ResolvedUser{
			ID:          "u1",
			Email:       "u1@example.com",
			DisplayName: "",
			IsAdmin:     false,
		},
		"kid-1",
		fosite.Arguments{OfflineAccessScope},
	)

	assert.Equal(t, "u1", sess.Subject)
	assert.Equal(t, "kid-1", sess.Headers.Extra["kid"])
	// Without the openid scope no identity claims are layered on.
	assert.Nil(t, sess.Claims.Extra["role"])
	assert.Nil(t, sess.Claims.Extra["email"])
}

func TestAddProfileClaims_ScopeMatrix(t *testing.T) {
	tests := []struct {
		name   string
		user   ResolvedUser
		scopes fosite.Arguments
		assert func(t *testing.T, extra map[string]any)
	}{
		{
			name: "non-admin maps to Viewer, openid only",
			user: ResolvedUser{
				ID: "u", Email: "v@example.com", DisplayName: "", IsAdmin: false,
			},
			scopes: fosite.Arguments{OpenIDScope},
			assert: func(t *testing.T, extra map[string]any) {
				t.Helper()
				assert.Equal(t, "Viewer", extra["role"])
				assert.NotContains(t, extra, "email")
				assert.NotContains(t, extra, "name")
			},
		},
		{
			name: "admin with email scope",
			user: ResolvedUser{
				ID: "u", Email: "a@example.com", DisplayName: "", IsAdmin: true,
			},
			scopes: fosite.Arguments{OpenIDScope, EmailScope},
			assert: func(t *testing.T, extra map[string]any) {
				t.Helper()
				assert.Equal(t, "Admin", extra["role"])
				assert.Equal(t, "a@example.com", extra["email"])
				assert.Equal(t, true, extra["email_verified"])
			},
		},
		{
			name: "profile scope falls back to email when no display name",
			user: ResolvedUser{
				ID: "u", Email: "p@example.com", DisplayName: "", IsAdmin: false,
			},
			scopes: fosite.Arguments{OpenIDScope, ProfileScope},
			assert: func(t *testing.T, extra map[string]any) {
				t.Helper()
				assert.Equal(t, "p@example.com", extra["name"])
				assert.Equal(t, "p@example.com", extra["preferred_username"])
			},
		},
		{
			name: "profile scope uses display name when set",
			user: ResolvedUser{
				ID: "u", Email: "p@example.com", DisplayName: "Pat", IsAdmin: false,
			},
			scopes: fosite.Arguments{OpenIDScope, ProfileScope},
			assert: func(t *testing.T, extra map[string]any) {
				t.Helper()
				assert.Equal(t, "Pat", extra["name"])
			},
		},
		{
			name: "email scope with empty email omits the claim",
			user: ResolvedUser{
				ID: "u", Email: "", DisplayName: "", IsAdmin: false,
			},
			scopes: fosite.Arguments{OpenIDScope, EmailScope},
			assert: func(t *testing.T, extra map[string]any) {
				t.Helper()
				assert.NotContains(t, extra, "email")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sess := newAuthorizeSession(tc.user, "kid", tc.scopes)
			require.NotNil(t, sess.Claims)
			tc.assert(t, sess.Claims.Extra)
		})
	}
}
