-- +goose Up
ALTER TABLE backlog.user_integrations
DROP COLUMN IF EXISTS steam_api_key,
DROP COLUMN IF EXISTS hardcover_api_key;

-- +goose Down
ALTER TABLE backlog.user_integrations
ADD COLUMN IF NOT EXISTS steam_api_key TEXT,
ADD COLUMN IF NOT EXISTS hardcover_api_key TEXT;
