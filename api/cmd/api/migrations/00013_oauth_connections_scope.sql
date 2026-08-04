-- +goose Up
-- +goose StatementBegin
ALTER TABLE global.oauth_connections ADD COLUMN IF NOT EXISTS scope TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE global.oauth_connections DROP COLUMN IF EXISTS scope;
-- +goose StatementEnd
