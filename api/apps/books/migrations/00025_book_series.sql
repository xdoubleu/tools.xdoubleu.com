-- +goose Up
-- +goose StatementBegin
-- New nullable columns, so no backfill: a forced resync fills them from
-- Hardcover. series_total caches Hardcover's primary-volume count.
ALTER TABLE books.books
ADD COLUMN series_name TEXT,
ADD COLUMN series_position DOUBLE PRECISION,
ADD COLUMN series_total INTEGER;

CREATE INDEX books_series_name_idx ON books.books (series_name)
WHERE series_name IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX books.books_series_name_idx;
ALTER TABLE books.books
DROP COLUMN series_name,
DROP COLUMN series_position,
DROP COLUMN series_total;
-- +goose StatementEnd
