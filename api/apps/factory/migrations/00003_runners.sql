-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS factory.runners (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    -- SHA-256 of the device token, set when the runner pairs.
    token_hash TEXT UNIQUE,
    -- SHA-256 of the single-use pairing code, cleared when it is used.
    pairing_hash TEXT UNIQUE,
    pairing_expires_at TIMESTAMPTZ,
    created_by TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    paired_at TIMESTAMPTZ,
    last_seen TIMESTAMPTZ,
    version TEXT NOT NULL DEFAULT '',
    revoked_at TIMESTAMPTZ
);

-- Only one runner at a time: a second active (paired or still pairing) row
-- conflicts until the first is revoked.
CREATE UNIQUE INDEX IF NOT EXISTS idx_runners_one_active
ON factory.runners ((TRUE)) WHERE revoked_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS factory.runners;
-- +goose StatementEnd
