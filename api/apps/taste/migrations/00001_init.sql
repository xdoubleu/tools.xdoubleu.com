-- +goose Up
-- +goose StatementBegin
CREATE SCHEMA IF NOT EXISTS taste;

-- One LLM-written profile per user. sections is NULL until the first
-- generation; pending marks a requested regeneration the job hasn't run;
-- attempted_at is the last generation, failed or not.
CREATE TABLE IF NOT EXISTS taste.profiles (
    user_id TEXT PRIMARY KEY,
    sections JSONB,
    model TEXT NOT NULL DEFAULT '',
    generated_at TIMESTAMPTZ,
    attempted_at TIMESTAMPTZ,
    requested_at TIMESTAMPTZ,
    pending BOOLEAN NOT NULL DEFAULT FALSE,
    last_error TEXT NOT NULL DEFAULT ''
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP SCHEMA IF EXISTS taste CASCADE;
-- +goose StatementEnd
