-- +goose Up
-- +goose StatementBegin
-- The streaming services a user subscribes to, as TMDB provider IDs.
CREATE TABLE IF NOT EXISTS movies.user_settings (
    user_id TEXT PRIMARY KEY,
    provider_ids INT [] NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS movies.user_settings;
-- +goose StatementEnd
