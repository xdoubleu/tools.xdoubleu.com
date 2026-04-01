-- +goose Up
-- +goose StatementBegin
ALTER TABLE goaltracker.steam_games
ADD COLUMN IF NOT EXISTS has_achievements boolean NOT NULL DEFAULT false;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE goaltracker.steam_games
DROP COLUMN IF EXISTS has_achievements;
-- +goose StatementEnd
