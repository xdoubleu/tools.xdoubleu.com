-- Queued notifications: a deploy doesn't drop mail, a failed send retries.
-- New table: nothing to backfill.

-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS global.notification_outbox (
    id BIGSERIAL PRIMARY KEY,
    recipient TEXT NOT NULL DEFAULT '',
    subject TEXT NOT NULL,
    body TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending', 'sent', 'failed')),
    attempts INT NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS notification_outbox_pending_idx
ON global.notification_outbox (next_attempt_at)
WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS notification_outbox_finished_idx
ON global.notification_outbox (finished_at)
WHERE finished_at IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS global.notification_outbox;
-- +goose StatementEnd
