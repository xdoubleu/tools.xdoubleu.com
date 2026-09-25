// Package oauthconn provides token refresh and CSRF-state plumbing shared by
// every OAuth-connected provider.
package oauthconn

import (
	"context"
	"errors"

	"golang.org/x/oauth2"

	"tools.xdoubleu.com/internal/database"
	"tools.xdoubleu.com/internal/models"
)

type connectionStore interface {
	Get(
		ctx context.Context,
		provider models.OAuthProvider,
	) (*oauth2.Token, *models.OAuthConnection, error)
	UpdateToken(
		ctx context.Context,
		provider models.OAuthProvider,
		tok *oauth2.Token,
	) error
}

// ErrNotConnected means no admin has connected the provider.
var ErrNotConnected = errors.New("oauthconn: provider not connected")

// TokenFunc returns a live bearer token, refreshing when expired.
type TokenFunc func(ctx context.Context) (string, error)

// NewTokenFunc reads the stored token, refreshes via TokenSource if needed, and
// persists the rotated token.
func NewTokenFunc(
	repo connectionStore, provider models.OAuthProvider, conf *oauth2.Config,
) TokenFunc {
	return func(ctx context.Context) (string, error) {
		tok, conn, err := repo.Get(ctx, provider)
		if errors.Is(err, database.ErrResourceNotFound) {
			return "", ErrNotConnected
		}
		if errors.Is(err, models.ErrDecryptFailed) {
			return "", models.ErrDecryptFailed
		}
		if err != nil {
			return "", err
		}
		if ScopesAreStale(conn, conf.Scopes) {
			// A refresh can't add scopes; only re-authorizing via Connect fixes this.
			return "", ErrNotConnected
		}

		fresh, err := conf.TokenSource(ctx, tok).Token()
		if err != nil {
			return "", err
		}

		if fresh.AccessToken != tok.AccessToken {
			// Best-effort: we already have a working token.
			_ = repo.UpdateToken(ctx, provider, fresh)
		}

		return fresh.AccessToken, nil
	}
}
