-- Pending passkey sign-ins. A discoverable-credential assertion starts before
-- the user is known, so the session is keyed by its challenge instead.

-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS auth.passkey_login_challenges (
    challenge TEXT PRIMARY KEY,
    session_data JSONB NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS passkey_login_challenges_expires_at_idx
ON auth.passkey_login_challenges (expires_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS auth.passkey_login_challenges;
-- +goose StatementEnd
