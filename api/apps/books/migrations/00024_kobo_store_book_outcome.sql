-- +goose Up
-- +goose StatementBegin
-- The last reading state the Kobo reported for a store book and what
-- mirroring it onto the library did. New columns: nothing to backfill.
ALTER TABLE books.kobo_store_books
ADD COLUMN last_percent INTEGER,
ADD COLUMN last_read_at TIMESTAMPTZ,
ADD COLUMN last_outcome TEXT NOT NULL DEFAULT '',
ADD COLUMN last_mirrored_at TIMESTAMPTZ;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE books.kobo_store_books
DROP COLUMN last_percent,
DROP COLUMN last_read_at,
DROP COLUMN last_outcome,
DROP COLUMN last_mirrored_at;
-- +goose StatementEnd
