-- +goose Up
-- +goose StatementBegin
-- Set when an admin deactivates the person (adr-0051). NULL means active, so
-- no backfill is needed.
ALTER TABLE auth.users ADD COLUMN IF NOT EXISTS deactivated_at TIMESTAMPTZ;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE auth.users DROP COLUMN IF EXISTS deactivated_at;
-- +goose StatementEnd
