-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS global.profile_shares (
    user_id TEXT PRIMARY KEY,
    token TEXT UNIQUE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS global.profile_shares;
-- +goose StatementEnd
