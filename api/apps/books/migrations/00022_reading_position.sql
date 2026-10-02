-- +goose Up
-- +goose StatementBegin
-- position is the format-neutral reading position: {"href","offset"} for
-- EPUB/KEPUB, {"page"} for PDF. location keeps its meaning (the Kobo span),
-- so no backfill: older rows resume from percent.
ALTER TABLE books.book_reading_state
ADD COLUMN position JSONB;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE books.book_reading_state
DROP COLUMN position;
-- +goose StatementEnd
