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
			// Reuse revokes the whole token family, so it alerts.
			name:      "refresh grant reuse detected is an error despite being a 4xx",
			grantType: refreshTokenGrant,
			rfcErr:    fosite.ErrInvalidGrant.WithWrap(fosite.ErrInactiveToken),
			want:      "ERROR",
		},
		{
			name:      "refresh grant plain expiry stays a warning",
			grantType: refreshTokenGrant,
			rfcErr:    fosite.ErrInvalidGrant.WithWrap(fosite.ErrTokenExpired),
			want:      "WARN",
		},
		{
			name:      "refresh grant not found stays a warning",
			grantType: refreshTokenGrant,
			rfcErr:    fosite.ErrInvalidGrant.WithWrap(fosite.ErrNotFound),
			want:      "WARN",
		},
		{
			// Escalation needs a positive reuse match.
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
	// Fosite rejected before resolving a client; must not panic.
	//nolint:exhaustruct //an empty request is exactly the case under test
	req := &fosite.Request{}
	req.Form = map[string][]string{"grant_type": {refreshTokenGrant}}

	grantType, clientID := requesterIdentity(req)
	assert.Equal(t, refreshTokenGrant, grantType)
	assert.Empty(t, clientID)
}

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
