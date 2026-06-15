-- +goose Up
-- +goose StatementBegin
ALTER TABLE backlog.user_books ADD COLUMN kobo_sync_enabled_at TIMESTAMPTZ;

-- Backfill existing rows that already have the kobo-sync tag so they get a
-- stable timestamp rather than NULL (use added_at as a reasonable proxy).
UPDATE backlog.user_books
SET kobo_sync_enabled_at = added_at
WHERE 'kobo-sync' = ANY(tags);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE backlog.user_books DROP COLUMN kobo_sync_enabled_at;
-- +goose StatementEnd
