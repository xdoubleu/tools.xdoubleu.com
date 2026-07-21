-- +goose Up
-- +goose StatementBegin
ALTER TABLE global.oauth_connections ADD COLUMN config JSONB;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE global.oauth_connections DROP COLUMN config;
-- +goose StatementEnd
