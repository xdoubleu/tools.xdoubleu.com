-- +goose Up
-- Indexes to support global deduplication by content checksum and
-- refcount-safe deletion by storage key.
CREATE INDEX IF NOT EXISTS idx_book_files_checksum
ON backlog.book_files (checksum);

CREATE INDEX IF NOT EXISTS idx_book_files_storage_key
ON backlog.book_files (storage_key);

-- +goose Down
DROP INDEX IF EXISTS backlog.idx_book_files_checksum;
DROP INDEX IF EXISTS backlog.idx_book_files_storage_key;
