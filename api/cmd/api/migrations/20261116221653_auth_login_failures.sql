-- Failed-credential counts and lock-until times, so a deploy or restart doesn't
-- reset an attacker's lockout. Keys are "signin:<email>" and "mfa:<userID>".
-- New table: nothing to backfill.

-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS auth.login_failures (
    key TEXT PRIMARY KEY,
    failures INT NOT NULL,
    last_failure TIMESTAMPTZ NOT NULL,
    locked_until TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS login_failures_last_failure_idx
ON auth.login_failures (last_failure);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS auth.login_failures;
-- +goose StatementEnd
