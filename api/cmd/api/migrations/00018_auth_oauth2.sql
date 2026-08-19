-- +goose Up
-- +goose StatementBegin
-- Embedded OAuth 2.1 authorization server (issue #1039) backing the MCP
-- server's OAuth flow via ory/fosite, replacing Supabase as the AS.
CREATE TABLE IF NOT EXISTS auth.oauth2_clients (
    id TEXT PRIMARY KEY,
    secret_hash TEXT,
    redirect_uris TEXT[] NOT NULL DEFAULT '{}',
    grant_types TEXT[] NOT NULL DEFAULT '{}',
    response_types TEXT[] NOT NULL DEFAULT '{}',
    scopes TEXT[] NOT NULL DEFAULT '{}',
    public BOOLEAN NOT NULL DEFAULT true,
    client_name TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS auth.oauth2_auth_codes (
    signature TEXT PRIMARY KEY,
    request JSONB NOT NULL,
    client_id TEXT NOT NULL REFERENCES auth.oauth2_clients (
        id
    ) ON DELETE CASCADE,
    user_id UUID,
    active BOOLEAN NOT NULL DEFAULT true,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS auth.oauth2_access_tokens (
    signature TEXT PRIMARY KEY,
    request JSONB NOT NULL,
    client_id TEXT NOT NULL REFERENCES auth.oauth2_clients (
        id
    ) ON DELETE CASCADE,
    user_id UUID,
    active BOOLEAN NOT NULL DEFAULT true,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS auth.oauth2_refresh_tokens (
    signature TEXT PRIMARY KEY,
    request JSONB NOT NULL,
    client_id TEXT NOT NULL REFERENCES auth.oauth2_clients (
        id
    ) ON DELETE CASCADE,
    user_id UUID,
    active BOOLEAN NOT NULL DEFAULT true,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS auth.oauth2_pkce_requests (
    signature TEXT PRIMARY KEY,
    request JSONB NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS auth.oauth2_pkce_requests;
DROP TABLE IF EXISTS auth.oauth2_refresh_tokens;
DROP TABLE IF EXISTS auth.oauth2_access_tokens;
DROP TABLE IF EXISTS auth.oauth2_auth_codes;
DROP TABLE IF EXISTS auth.oauth2_clients;
-- +goose StatementEnd
