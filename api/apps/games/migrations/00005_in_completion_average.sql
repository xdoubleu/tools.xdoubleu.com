-- +goose Up
-- +goose StatementBegin
ALTER TABLE games.steam_games
ADD COLUMN in_completion_average BOOLEAN NOT NULL DEFAULT TRUE;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE games.steam_games
DROP COLUMN in_completion_average;
-- +goose StatementEnd
