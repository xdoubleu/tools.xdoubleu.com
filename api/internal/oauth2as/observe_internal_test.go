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
		code      int
		want      string
	}{
		{
			name:      "server fault is always an error",
			grantType: authorizationCodeGrant,
			code:      http.StatusInternalServerError,
			want:      "ERROR",
		},
		{
			// The whole point of the policy: a client that held a working
			// session just lost it (issue #1177).
			name:      "failed refresh grant is an error despite being a 4xx",
			grantType: refreshTokenGrant,
			code:      http.StatusBadRequest,
			want:      "ERROR",
		},
		{
			name:      "routine 4xx stays a warning",
			grantType: authorizationCodeGrant,
			code:      http.StatusBadRequest,
			want:      "WARN",
		},
		{
			// No grant_type at all — the /oauth2/authorize leg, or a request
			// fosite rejected before parsing the form.
			name:      "missing grant type stays a warning",
			grantType: "",
			code:      http.StatusUnauthorized,
			want:      "WARN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			//nolint:exhaustruct //only CodeField drives oauthErrorLevel
			rfcErr := &fosite.RFC6749Error{CodeField: tt.code}
			assert.Equal(
				t, tt.want, oauthErrorLevel(tt.grantType, rfcErr).String(),
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
