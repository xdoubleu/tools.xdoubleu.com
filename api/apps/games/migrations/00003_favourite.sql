-- +goose Up
-- +goose StatementBegin
ALTER TABLE games.steam_games
ADD COLUMN favourite BOOLEAN NOT NULL DEFAULT FALSE;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE games.steam_games
DROP COLUMN favourite;
-- +goose StatementEnd
