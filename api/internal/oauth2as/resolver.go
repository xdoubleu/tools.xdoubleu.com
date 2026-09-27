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
	_, requester, err := t.provider.IntrospectToken(
		ctx, token, fosite.AccessToken, session,
	)
	if err != nil {
		return "", err
	}
	// Grafana's SSO tokens are for Grafana, not the MCP resource server.
	if requester.GetClient().GetID() == GrafanaClientID {
		return "", errors.New("oauth2as: token was issued to another client")
	}
	if session.Subject == "" {
		return "", errors.New("oauth2as: token has no subject")
	}
	return session.Subject, nil
}
