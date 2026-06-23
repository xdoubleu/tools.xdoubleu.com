-- +goose Up
-- +goose StatementBegin
ALTER TABLE backlog.steam_games
ADD COLUMN last_synced_at TIMESTAMPTZ NOT NULL DEFAULT now();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE backlog.steam_games DROP COLUMN last_synced_at;
-- +goose StatementEnd
