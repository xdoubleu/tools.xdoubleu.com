-- +goose Up
-- +goose StatementBegin
-- When picks were last asked for, by a picks or a profile refresh; a request
-- is pending until an attempt starts after it. No backfill: NULL means never
-- requested.
ALTER TABLE taste.profiles
ADD COLUMN IF NOT EXISTS recommendations_requested_at TIMESTAMPTZ;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE taste.profiles
DROP COLUMN IF EXISTS recommendations_requested_at;
-- +goose StatementEnd
