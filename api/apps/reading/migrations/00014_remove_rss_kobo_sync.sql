-- +goose Up
-- +goose StatementBegin
-- Issue #640: RSS items never sync to Kobo devices. Clear any stale
-- kobo-sync state on already-ingested RSS items before dropping the
-- per-feed auto-opt-in flag that used to set it.
UPDATE reading.user_books ub
SET
    tags = array_remove(ub.tags, 'kobo-sync'),
    kobo_sync_enabled_at = NULL
FROM reading.books AS b
WHERE b.id = ub.book_id AND b.category = 'rss' AND 'kobo-sync' = any(ub.tags);

ALTER TABLE reading.feeds DROP COLUMN kobo_sync;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE reading.feeds ADD COLUMN kobo_sync BOOLEAN NOT NULL DEFAULT FALSE;
-- +goose StatementEnd
