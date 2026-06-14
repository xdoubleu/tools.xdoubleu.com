-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS backlog.kobo_devices (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id text NOT NULL,
    token_hash text NOT NULL UNIQUE,
    name text NOT NULL,
    serial text,
    created_at timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz
);
CREATE INDEX IF NOT EXISTS idx_kobo_devices_user_id ON backlog.kobo_devices (
    user_id
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS backlog.kobo_devices;
-- +goose StatementEnd
