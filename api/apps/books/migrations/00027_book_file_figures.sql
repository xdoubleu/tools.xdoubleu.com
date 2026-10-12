-- +goose Up
-- +goose StatementBegin
-- New nullable columns, so no backfill: the figure-analysis job fills them.
-- analysed_pages = 0 marks a file that couldn't be analysed.
ALTER TABLE books.book_files
ADD COLUMN figure_count INTEGER,
ADD COLUMN analysed_pages INTEGER;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE books.book_files
DROP COLUMN figure_count,
DROP COLUMN analysed_pages;
-- +goose StatementEnd
