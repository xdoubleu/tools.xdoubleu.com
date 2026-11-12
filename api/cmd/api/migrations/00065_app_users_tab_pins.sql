-- +goose Up
-- +goose StatementBegin
-- The apps a user pinned to the phone tab bar, in order. NULL means never
-- chosen (the web falls back to its default pins), so no backfill is needed.
ALTER TABLE global.app_users ADD COLUMN IF NOT EXISTS tab_pins TEXT[];
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE global.app_users DROP COLUMN IF EXISTS tab_pins;
-- +goose StatementEnd
