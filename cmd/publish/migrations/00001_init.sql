-- +goose Up
-- +goose StatementBegin
CREATE SCHEMA IF NOT EXISTS global;

CREATE TABLE IF NOT EXISTS global.app_users (
    id TEXT NOT NULL PRIMARY KEY,
    email TEXT NOT NULL,
    last_seen TIMESTAMPTZ NOT NULL DEFAULT now(),
    role TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('admin', 'user'))
);

CREATE TABLE IF NOT EXISTS global.app_access (
    user_id TEXT NOT NULL REFERENCES global.app_users (id) ON DELETE CASCADE,
    app_name TEXT NOT NULL,
    PRIMARY KEY (user_id, app_name)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS global.app_access;
DROP TABLE IF EXISTS global.app_users;
DROP SCHEMA IF EXISTS global;
-- +goose StatementEnd
