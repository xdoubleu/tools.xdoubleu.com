package oauth2as

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/url"
	"strings"

	"github.com/ory/fosite"

	"tools.xdoubleu.com/internal/database/postgres"
)

// ClientMetadata is the subset of RFC 7591 dynamic client registration
// fields this server accepts.
type ClientMetadata struct {
	RedirectURIs []string `json:"redirect_uris"`
	ClientName   string   `json:"client_name"`
}

const clientIDBytes = 16

// RegisterClient implements RFC 7591 dynamic client registration for a
// public client (token_endpoint_auth_method is always forced to "none" —
// this authorization server never issues confidential-client secrets).
func RegisterClient(
	ctx context.Context,
	db postgres.DB,
	metadata ClientMetadata,
) (*fosite.DefaultClient, error) {
	if len(metadata.RedirectURIs) == 0 {
		return nil, errors.New("oauth2as: redirect_uris is required")
	}
	for _, uri := range metadata.RedirectURIs {
		if err := validateRedirectURI(uri); err != nil {
			return nil, err
		}
	}

	id, err := generateClientID()
	if err != nil {
		return nil, err
	}

	client := &fosite.DefaultClient{
		ID:             id,
		Secret:         nil,
		RotatedSecrets: nil,
		RedirectURIs:   metadata.RedirectURIs,
		GrantTypes:     []string{"authorization_code", "refresh_token"},
		ResponseTypes:  []string{"code"},
		Scopes:         []string{"offline_access"},
		Audience:       nil,
		Public:         true,
	}

	_, err = db.Exec(ctx, `
		INSERT INTO auth.oauth2_clients
			(id, secret_hash, redirect_uris, grant_types,
			 response_types, scopes, public, client_name)
		VALUES ($1, NULL, $2, $3, $4, $5, true, $6)
	`,
		client.ID, client.RedirectURIs, client.GrantTypes, client.ResponseTypes,
		client.Scopes, metadata.ClientName,
	)
	if err != nil {
		return nil, err
	}

	return client, nil
}

// validateRedirectURI requires HTTPS, except for localhost/127.0.0.1 during
// development (loopback redirect URIs are the documented OAuth 2.1 exception
// for native/local clients, e.g. an MCP CLI callback server).
func validateRedirectURI(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.Scheme == "https" {
		return nil
	}
	host := strings.Split(u.Host, ":")[0]
	if u.Scheme == "http" && (host == "localhost" || host == "127.0.0.1") {
		return nil
	}
	return errors.New("oauth2as: redirect_uris must be HTTPS (or localhost for dev)")
}

func generateClientID() (string, error) {
	buf := make([]byte, clientIDBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
