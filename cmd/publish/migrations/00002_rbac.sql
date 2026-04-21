-- +goose Up
-- +goose StatementBegin
ALTER TABLE global.app_users
ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT 'user'
CHECK (role IN ('admin', 'user'));

CREATE TABLE IF NOT EXISTS global.app_access (
    user_id TEXT NOT NULL REFERENCES global.app_users (id) ON DELETE CASCADE,
    app_name TEXT NOT NULL,
    PRIMARY KEY (user_id, app_name)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS global.app_access;
ALTER TABLE global.app_users DROP COLUMN IF EXISTS role;
-- +goose StatementEnd
