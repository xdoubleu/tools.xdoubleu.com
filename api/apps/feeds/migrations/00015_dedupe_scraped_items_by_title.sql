-- +goose Up
-- +goose StatementBegin
-- Scraped sites republish one post under several URLs; keep the earliest
-- stored copy per (feed, title).
DELETE FROM feeds.items AS i
USING feeds.feeds AS f
WHERE
    i.feed_id = f.id
    AND f.source_type = 'scrape'
    AND btrim(i.title) <> ''
    AND EXISTS (
        SELECT 1 FROM feeds.items AS k
        WHERE
            k.feed_id = i.feed_id
            AND lower(btrim(k.title)) = lower(btrim(i.title))
            AND (k.created_at, k.id) < (i.created_at, i.id)
    );
-- +goose StatementEnd

-- +goose Down
-- Not reversible: deleted duplicate rows are not recoverable.
-- +goose StatementBegin
SELECT 1;
-- +goose StatementEnd
