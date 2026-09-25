package oauth2as

import (
	"context"
	"errors"

	"github.com/ory/fosite"
)

// TokenResolver implements auth.OAuth2TokenResolver so internal/auth never
// imports fosite.
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
