-- +goose Up
-- +goose StatementBegin
-- A rotated client secret keeps working until previous_secret_expires_at
-- (adr-0047), so an agent's stored credential can be swapped without downtime.
ALTER TABLE auth.oauth2_clients
ADD COLUMN IF NOT EXISTS previous_secret_hash TEXT,
ADD COLUMN IF NOT EXISTS previous_secret_expires_at TIMESTAMPTZ;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE auth.oauth2_clients
DROP COLUMN IF EXISTS previous_secret_hash,
DROP COLUMN IF EXISTS previous_secret_expires_at;
-- +goose StatementEnd
