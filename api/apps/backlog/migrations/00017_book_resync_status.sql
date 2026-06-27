-- +goose Up
ALTER TABLE backlog.books
ADD COLUMN IF NOT EXISTS openlibrary_found BOOLEAN,
ADD COLUMN IF NOT EXISTS googlebooks_found BOOLEAN,
ADD COLUMN IF NOT EXISTS last_resync_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE backlog.books
DROP COLUMN IF EXISTS openlibrary_found,
DROP COLUMN IF EXISTS googlebooks_found,
DROP COLUMN IF EXISTS last_resync_at;
