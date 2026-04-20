-- +goose Up
-- +goose StatementBegin
CREATE SCHEMA IF NOT EXISTS global;

CREATE TABLE IF NOT EXISTS global.app_users (
    id TEXT NOT NULL PRIMARY KEY,
    email TEXT NOT NULL,
    last_seen TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS global.app_users;
DROP SCHEMA IF EXISTS global;
-- +goose StatementEnd
