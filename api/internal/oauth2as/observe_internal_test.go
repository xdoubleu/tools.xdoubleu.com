package oauth2as

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/ory/fosite"
	"github.com/stretchr/testify/assert"
)

func TestOAuthErrorLevel(t *testing.T) {
	tests := []struct {
		name      string
		grantType string
		rfcErr    *fosite.RFC6749Error
		want      string
	}{
		{
			name:      "server fault is always an error",
			grantType: authorizationCodeGrant,
			//nolint:exhaustruct //only CodeField drives this case
			rfcErr: &fosite.RFC6749Error{CodeField: http.StatusInternalServerError},
			want:   "ERROR",
		},
		{
			// The whole point of the policy: fosite's theft-response to a
			// replayed refresh token revokes the entire token family, not
			// just this one request — a client that held a working session
			// just lost it (issue #1177).
			name:      "refresh grant reuse detected is an error despite being a 4xx",
			grantType: refreshTokenGrant,
			rfcErr:    fosite.ErrInvalidGrant.WithWrap(fosite.ErrInactiveToken),
			want:      "ERROR",
		},
		{
			// A plain expired refresh token is routine housekeeping (issue
			// #1715) — the expected outcome of a session nobody kept alive,
			// not evidence one just broke.
			name:      "refresh grant plain expiry stays a warning",
			grantType: refreshTokenGrant,
			rfcErr:    fosite.ErrInvalidGrant.WithWrap(fosite.ErrTokenExpired),
			want:      "WARN",
		},
		{
			// A refresh token fosite never finds — malformed, tampered,
			// already deleted by a prior revocation, or a scanner probing
			// /oauth2/token with a self-registered client_id — carries no
			// more signal than any other stranger hitting the endpoint
			// (issue #1715).
			name:      "refresh grant not found stays a warning",
			grantType: refreshTokenGrant,
			rfcErr:    fosite.ErrInvalidGrant.WithWrap(fosite.ErrNotFound),
			want:      "WARN",
		},
		{
			// No cause at all still defaults to a warning — the escalation
			// requires a *positive* reuse-detected match, not merely being a
			// refresh_token grant.
			name:      "refresh grant with no identifiable cause stays a warning",
			grantType: refreshTokenGrant,
			//nolint:exhaustruct //only CodeField drives this case
			rfcErr: &fosite.RFC6749Error{CodeField: http.StatusBadRequest},
			want:   "WARN",
		},
		{
			name:      "routine 4xx stays a warning",
			grantType: authorizationCodeGrant,
			//nolint:exhaustruct //only CodeField drives this case
			rfcErr: &fosite.RFC6749Error{CodeField: http.StatusBadRequest},
			want:   "WARN",
		},
		{
			// No grant_type at all — the /oauth2/authorize leg, or a request
			// fosite rejected before parsing the form.
			name:      "missing grant type stays a warning",
			grantType: "",
			//nolint:exhaustruct //only CodeField drives this case
			rfcErr: &fosite.RFC6749Error{CodeField: http.StatusUnauthorized},
			want:   "WARN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(
				t, tt.want, oauthErrorLevel(tt.grantType, tt.rfcErr).String(),
			)
		})
	}
}

func TestRequesterIdentity_NilRequester(t *testing.T) {
	grantType, clientID := requesterIdentity(nil)
	assert.Empty(t, grantType)
	assert.Empty(t, clientID)
}

func TestRequesterIdentity_NoClientResolvedYet(t *testing.T) {
	// fosite hands back a requester with no client when it rejected the
	// request before resolving one — the identity helper must not panic.
	//nolint:exhaustruct //an empty request is exactly the case under test
	req := &fosite.Request{}
	req.Form = map[string][]string{"grant_type": {refreshTokenGrant}}

	grantType, clientID := requesterIdentity(req)
	assert.Equal(t, refreshTokenGrant, grantType)
	assert.Empty(t, clientID)
}

// TestLogHelpers_NilLoggerIsANoOp covers the guard both helpers open with:
// the handlers are constructible without a logger (tests and any future
// caller that doesn't wire one), and that must not panic.
func TestLogHelpers_NilLoggerIsANoOp(t *testing.T) {
	ctx := context.Background()
	err := errors.New("boom")

	assert.NotPanics(t, func() {
		logOAuthError(ctx, nil, endpointToken, nil, err)
	})
	assert.NotPanics(t, func() {
		logRegisterError(ctx, nil, err)
	})
}
