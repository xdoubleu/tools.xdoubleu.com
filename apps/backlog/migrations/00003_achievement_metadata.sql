-- +goose Up
-- +goose StatementBegin
ALTER TABLE backlog.steam_achievements
ADD COLUMN IF NOT EXISTS display_name TEXT NOT NULL DEFAULT '',
ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '',
ADD COLUMN IF NOT EXISTS global_percent NUMERIC(6, 2);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE backlog.steam_achievements
DROP COLUMN IF EXISTS display_name,
DROP COLUMN IF EXISTS description,
DROP COLUMN IF EXISTS global_percent;
-- +goose StatementEnd
