-- +goose Up
-- +goose StatementBegin
DROP TABLE IF EXISTS goaltracker.goals;
DROP TABLE IF EXISTS goaltracker.states;

ALTER TABLE goaltracker.user_integrations
DROP COLUMN IF EXISTS todoist_api_key,
DROP COLUMN IF EXISTS todoist_project_id;

ALTER TABLE goaltracker.steam_games
ADD COLUMN IF NOT EXISTS playtime_forever INTEGER NOT NULL DEFAULT 0;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE goaltracker.steam_games DROP COLUMN IF EXISTS playtime_forever;

ALTER TABLE goaltracker.user_integrations
ADD COLUMN IF NOT EXISTS todoist_api_key TEXT NOT NULL DEFAULT '',
ADD COLUMN IF NOT EXISTS todoist_project_id TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd
