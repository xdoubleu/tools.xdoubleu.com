-- +goose Up
-- +goose StatementBegin
-- expired_at hides an unread item that outlived its feed's lifetime; the
-- first job run expires the whole existing backlog on purpose (restorable),
-- so there is no backfill. expiry_days 0 means never.
ALTER TABLE feeds.items ADD COLUMN expired_at TIMESTAMPTZ;
ALTER TABLE feeds.feeds ADD COLUMN expiry_days INT NOT NULL DEFAULT 7
CHECK (expiry_days >= 0);

CREATE INDEX idx_items_expirable ON feeds.items (created_at)
WHERE read_at IS NULL AND expired_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS feeds.idx_items_expirable;
ALTER TABLE feeds.feeds DROP COLUMN expiry_days;
ALTER TABLE feeds.items DROP COLUMN expired_at;
-- +goose StatementEnd
