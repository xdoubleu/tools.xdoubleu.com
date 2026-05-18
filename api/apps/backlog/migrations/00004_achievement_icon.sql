-- +goose Up
-- +goose StatementBegin
ALTER TABLE backlog.steam_achievements
ADD COLUMN IF NOT EXISTS icon_url TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE backlog.steam_achievements
DROP COLUMN IF EXISTS icon_url;
-- +goose StatementEnd
