-- +goose Up
-- +goose StatementBegin
-- OpenID Connect explicit-flow session storage (issue #1469): the embedded
-- authorization server (internal/oauth2as) now issues ID tokens for clients
-- that request the openid scope. fosite's OpenIDConnectExplicitHandler keeps
-- the authorize-time request — with the fully-populated ID-token claims — in
-- its own store keyed by the authorization code, separate from
-- oauth2_auth_codes, and reads it back at the token endpoint to mint the ID
-- token. Rows are deleted as the code is redeemed; a stale row is bounded by
-- expires_at (the authorization-code lifespan).
CREATE TABLE IF NOT EXISTS auth.oauth2_oidc_sessions (
    signature TEXT PRIMARY KEY,
    request JSONB NOT NULL,
    client_id TEXT NOT NULL REFERENCES auth.oauth2_clients (
        id
    ) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS auth.oauth2_oidc_sessions;
-- +goose StatementEnd
