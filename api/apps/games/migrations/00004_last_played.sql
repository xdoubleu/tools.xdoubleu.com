-- +goose Up
-- +goose StatementBegin
ALTER TABLE games.steam_games
ADD COLUMN last_played TIMESTAMPTZ;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE games.steam_games
DROP COLUMN last_played;
-- +goose StatementEnd
