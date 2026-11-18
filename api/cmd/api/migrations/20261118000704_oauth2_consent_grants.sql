-- Remembered consent: a client opts in with remember_consent, and an approved
-- (user, client) scope set then skips the consent page while it still covers
-- the request. New table and column default to off: nothing to backfill.

-- +goose Up
-- +goose StatementBegin
ALTER TABLE auth.oauth2_clients
ADD COLUMN IF NOT EXISTS remember_consent BOOLEAN NOT NULL DEFAULT FALSE;

UPDATE auth.oauth2_clients
SET remember_consent = TRUE
WHERE id = 'grafana';

CREATE TABLE IF NOT EXISTS auth.oauth2_consent_grants (
    user_id UUID NOT NULL REFERENCES auth.users (id) ON DELETE CASCADE,
    client_id TEXT NOT NULL REFERENCES auth.oauth2_clients (
        id
    ) ON DELETE CASCADE,
    scopes TEXT[] NOT NULL,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, client_id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS auth.oauth2_consent_grants;
ALTER TABLE auth.oauth2_clients DROP COLUMN IF EXISTS remember_consent;
-- +goose StatementEnd
