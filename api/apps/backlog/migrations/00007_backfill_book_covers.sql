-- +goose Up
-- Backfill cover_url for books imported via CSV that have an ISBN13 but no
-- cover. Uses OpenLibrary's ISBN cover endpoint which requires no API key.
UPDATE backlog.books
SET cover_url = 'https://covers.openlibrary.org/b/isbn/' || isbn13 || '-L.jpg'
WHERE cover_url IS NULL AND isbn13 IS NOT NULL;

-- +goose Down
-- Cannot safely distinguish backfilled covers from genuine ones; no-op.
