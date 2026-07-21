// Package oauthconn provides the shared "fetch a live token, refreshing
// transparently" and CSRF-state plumbing used by every OAuth-connected
// external provider (GitHub, Sentry, DigitalOcean). Provider-specific
// endpoints/scopes live next to each provider's client instead of here.
package oauthconn

import (
	"context"
	"errors"

	"github.com/xdoubleu/essentia/v4/pkg/database"
	"golang.org/x/oauth2"

	"tools.xdoubleu.com/internal/models"
)

// connectionStore is the subset of *repositories.OAuthConnectionsRepository
// this package depends on, so tests can stub it without a database.
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

// ErrNotConnected is returned by a TokenFunc when no admin has connected the
// provider yet.
var ErrNotConnected = errors.New("oauthconn: provider not connected")

// TokenFunc returns a live bearer token for a request, refreshing it via the
// provider's oauth2.Config when the stored token is expired.
type TokenFunc func(ctx context.Context) (string, error)

// NewTokenFunc builds a TokenFunc for provider: it reads the stored token,
// lets oauth2.Config.TokenSource refresh it if needed, and persists the
// rotated token back to repo so the refresh only happens once.
func NewTokenFunc(
	repo connectionStore, provider models.OAuthProvider, conf *oauth2.Config,
) TokenFunc {
	return func(ctx context.Context) (string, error) {
		tok, _, err := repo.Get(ctx, provider)
		if errors.Is(err, database.ErrResourceNotFound) {
			return "", ErrNotConnected
		}
		if err != nil {
			return "", err
		}

		fresh, err := conf.TokenSource(ctx, tok).Token()
		if err != nil {
			return "", err
		}

		if fresh.AccessToken != tok.AccessToken {
			// Best-effort: a persistence hiccup shouldn't fail the caller when
			// we already have a working token in hand.
			_ = repo.UpdateToken(ctx, provider, fresh)
		}

		return fresh.AccessToken, nil
	}
}
