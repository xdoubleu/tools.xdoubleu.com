-- +goose Up
-- +goose StatementBegin
-- Per-user taste settings. A user without a row gets the defaults, so no
-- backfill is needed: email_new_picks is on unless they turn it off.
CREATE TABLE IF NOT EXISTS taste.preferences (
    user_id TEXT PRIMARY KEY,
    email_new_picks BOOLEAN NOT NULL DEFAULT TRUE
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS taste.preferences;
-- +goose StatementEnd
