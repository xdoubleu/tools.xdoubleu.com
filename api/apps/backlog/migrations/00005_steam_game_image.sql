-- +goose Up
ALTER TABLE backlog.steam_games
ADD COLUMN IF NOT EXISTS image_url TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE backlog.steam_games
DROP COLUMN IF EXISTS image_url;
