-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS goaltracker.user_integrations (
    user_id TEXT NOT NULL PRIMARY KEY,
    todoist_api_key TEXT NOT NULL DEFAULT '',
    todoist_project_id TEXT NOT NULL DEFAULT '',
    steam_api_key TEXT NOT NULL DEFAULT '',
    steam_user_id TEXT NOT NULL DEFAULT '',
    goodreads_url TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS goaltracker.user_integrations;
-- +goose StatementEnd
