-- +goose Up
-- +goose StatementBegin
CREATE SCHEMA icsproxy;

CREATE TABLE icsproxy.feeds (
    token TEXT PRIMARY KEY,
    source_url TEXT NOT NULL,
    hide_event_uids TEXT [] NOT NULL DEFAULT '{}',
    holiday_uids TEXT [] NOT NULL DEFAULT '{}',
    hide_series JSONB NOT NULL DEFAULT '{}'::JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP SCHEMA IF EXISTS icsproxy CASCADE;
-- +goose StatementEnd
