-- +goose Up
-- +goose StatementBegin
ALTER TABLE global.app_users ADD COLUMN IF NOT EXISTS display_name TEXT;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE IF EXISTS global.profile_shares;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS global.profile_shares (
    user_id TEXT NOT NULL,
    app TEXT NOT NULL CHECK (app IN ('books', 'games')),
    token TEXT UNIQUE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, app)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS global.profile_shares;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS global.profile_shares (
    user_id TEXT PRIMARY KEY,
    token TEXT UNIQUE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE global.app_users DROP COLUMN IF EXISTS display_name;
-- +goose StatementEnd
