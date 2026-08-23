-- +goose Up
-- +goose StatementBegin
ALTER TABLE global.oauth_connections
ADD COLUMN IF NOT EXISTS requested_scope TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE global.oauth_connections DROP COLUMN IF EXISTS requested_scope;
-- +goose StatementEnd
