-- +goose Up
-- +goose StatementBegin
-- Backfill: dropped/owned were never registered in books.shelves (see
-- registerCustomShelf), so a user currently on either shelf would lose it
-- the next time it's emptied. Register anyone with a book there now.
INSERT INTO books.shelves (user_id, name)
SELECT DISTINCT
    user_id,
    status
FROM books.user_books
WHERE status IN ('dropped', 'owned')
ON CONFLICT DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 1; -- no-op: data backfill, nothing structural to revert
-- +goose StatementEnd
