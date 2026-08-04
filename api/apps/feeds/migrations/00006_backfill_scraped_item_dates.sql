-- Issue #829: before the scrape.go fix, discoverPostLinks concatenated a
-- scraped card's entire anchor text into the item title, so a "Mon D, YYYY"
-- date (sometimes with a leading category label) ended up embedded in the
-- title, while published_at was always set to ingest time. Backfill
-- existing scrape-sourced items: pull the date out of the title into
-- published_at, and strip it from the title text.
--
-- ponytail: only strips the recognized date substring — the leftover
-- category-label/excerpt noise on "featured card" titles (e.g. "Societal
-- Impacts ...") isn't recoverable from the flattened stored title and is
-- left as-is; re-scraping would be needed to fully clean those up.

-- +goose Up
-- +goose StatementBegin
UPDATE feeds.items i
SET
    published_at = to_date(m.date_text, 'Mon DD, YYYY'),
    title = btrim(regexp_replace(
        regexp_replace(i.title, m.date_text, '', 'g'), '\s{2,}', ' ', 'g'
    ))
FROM (
    SELECT
        si.id,
        (regexp_match(si.title, '[A-Z][a-z]{2} \d{1,2}, \d{4}'))[1] AS date_text
    FROM feeds.items AS si
    INNER JOIN feeds.feeds AS f ON si.feed_id = f.id
    WHERE f.source_type = 'scrape'
) AS m
WHERE i.id = m.id AND m.date_text IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- Not reversible: the original titles/published_at are not recoverable.
-- +goose StatementBegin
SELECT 1;
-- +goose StatementEnd
