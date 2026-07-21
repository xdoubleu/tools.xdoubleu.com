-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS global.oauth_connections (
    provider TEXT PRIMARY KEY CHECK (
        provider IN ('github', 'sentry', 'digitalocean')
    ),
    access_token BYTEA NOT NULL,
    refresh_token BYTEA,
    expires_at TIMESTAMPTZ,
    connected_by TEXT NOT NULL,
    connected_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS global.oauth_connections;
-- +goose StatementEnd
