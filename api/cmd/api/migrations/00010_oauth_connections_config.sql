-- +goose Up
-- +goose StatementBegin
ALTER TABLE global.oauth_connections ADD COLUMN IF NOT EXISTS config JSONB;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE global.oauth_connections DROP COLUMN IF EXISTS config;
-- +goose StatementEnd
