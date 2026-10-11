-- +goose Up
-- +goose StatementBegin
-- The physical copy's details, for mapping its pages to reading positions.
-- New nullable columns, so no backfill: NULL falls back to the catalog's
-- page count and page 1.
ALTER TABLE books.user_books
ADD COLUMN physical_page_count INTEGER CHECK (physical_page_count > 0),
ADD COLUMN physical_text_start_page INTEGER CHECK (
    physical_text_start_page > 0
),
ADD COLUMN physical_anchors JSONB NOT NULL DEFAULT '[]';

ALTER TABLE books.book_reading_state
DROP CONSTRAINT book_reading_state_source_check,
ADD CONSTRAINT book_reading_state_source_check CHECK (
    source IN ('web', 'kobo', 'manual', 'physical')
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM books.book_reading_state WHERE source = 'physical';

ALTER TABLE books.book_reading_state
DROP CONSTRAINT book_reading_state_source_check,
ADD CONSTRAINT book_reading_state_source_check CHECK (
    source IN ('web', 'kobo', 'manual')
);

ALTER TABLE books.user_books
DROP COLUMN physical_page_count,
DROP COLUMN physical_text_start_page,
DROP COLUMN physical_anchors;
-- +goose StatementEnd
