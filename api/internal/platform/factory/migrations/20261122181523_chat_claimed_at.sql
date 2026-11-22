-- +goose Up
-- +goose StatementBegin
-- When the pinned runner took the session over; null until it does. A runner
-- takes a session once: a session it lost its VM for ends by missed check-ins.
-- New column, nothing to backfill: sessions that exist before it are claimable.
ALTER TABLE factory.chat_sessions
ADD COLUMN claimed_at TIMESTAMPTZ;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE factory.chat_sessions DROP COLUMN IF EXISTS claimed_at;
-- +goose StatementEnd
