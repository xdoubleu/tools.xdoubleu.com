package oauth2as

import (
	"context"
	"errors"

	"github.com/ory/fosite"
)

// TokenResolver adapts an OAuth2Provider into auth.OAuth2TokenResolver
// (internal/auth), letting the auth middleware resolve a fosite-issued
// opaque access token to the user ID it was granted for. Defined here
// (rather than internal/auth wiring it directly against fosite) to avoid
// internal/auth needing to import fosite at all — cmd/api, the composition
// root, wires the two packages together via internal/auth's
// OAuth2TokenResolver interface.
type TokenResolver struct {
	provider fosite.OAuth2Provider
}

func NewTokenResolver(provider fosite.OAuth2Provider) *TokenResolver {
	return &TokenResolver{provider: provider}
}

func (t *TokenResolver) ResolveAccessToken(
	ctx context.Context,
	token string,
) (string, error) {
	//nolint:exhaustruct //other DefaultSession fields are optional
	session := &fosite.DefaultSession{}
	_, _, err := t.provider.IntrospectToken(
		ctx, token, fosite.AccessToken, session,
	)
	if err != nil {
		return "", err
	}
	if session.Subject == "" {
		return "", errors.New("oauth2as: token has no subject")
	}
	return session.Subject, nil
}
