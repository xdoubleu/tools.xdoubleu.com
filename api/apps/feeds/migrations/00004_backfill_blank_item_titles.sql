-- Issue #763: buildItem/IngestEmail stored blank/whitespace-only titles for
-- items ingested before this was fixed in code. Backfill those rows: email
-- items (source_url starts with "mailto:") get the same default IngestEmail
-- now applies for a blank subject; scraped/RSS items fall back to their
-- source_url, mirroring buildItem's own canonical-URL fallback.

-- +goose Up
-- +goose StatementBegin
UPDATE feeds.items
SET
    title = CASE
        WHEN source_url LIKE 'mailto:%' THEN 'Email newsletter'
        ELSE source_url
    END
WHERE btrim(title) = '';
-- +goose StatementEnd

-- +goose Down
-- Not reversible: the original blank titles are not recoverable.
-- +goose StatementBegin
SELECT 1;
-- +goose StatementEnd
