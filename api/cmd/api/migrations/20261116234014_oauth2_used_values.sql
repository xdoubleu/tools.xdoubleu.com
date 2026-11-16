-- Replay records for PKCE challenges, OIDC nonces and client assertion JTIs;
-- only a digest of the value is kept. New table: nothing to backfill.

-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS auth.oauth2_used_values (
    kind TEXT NOT NULL,
    client_id TEXT NOT NULL,
    digest TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (kind, client_id, digest)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS auth.oauth2_used_values;
-- +goose StatementEnd
