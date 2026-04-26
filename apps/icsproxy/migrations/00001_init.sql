-- +goose Up
-- +goose StatementBegin
CREATE SCHEMA IF NOT EXISTS icsproxy;

CREATE TABLE IF NOT EXISTS icsproxy.feeds (
    token_hash TEXT PRIMARY KEY,
    source_url TEXT NOT NULL,
    hide_event_uids TEXT [] NOT NULL DEFAULT '{}',
    holiday_uids TEXT [] NOT NULL DEFAULT '{}',
    hide_series JSONB NOT NULL DEFAULT '{}'::JSONB,
    user_id TEXT NOT NULL REFERENCES global.app_users (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP SCHEMA IF EXISTS icsproxy CASCADE;
-- +goose StatementEnd
