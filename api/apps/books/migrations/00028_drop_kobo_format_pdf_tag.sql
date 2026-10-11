-- +goose Up
-- +goose StatementBegin
-- The Kobo store sync never downloads a PDF, so the raw-PDF preference is
-- gone and every kobo-sync book goes out as its KEPUB.
UPDATE books.user_books
SET tags = array_remove(tags, 'kobo-format-pdf')
WHERE 'kobo-format-pdf' = any(tags);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 1; -- no-op: the dropped preference can't be restored
-- +goose StatementEnd
