-- The reading app now tracks more than books: arXiv papers, manually added
-- web articles, and RSS-ingested posts. Every catalog row gets a fixed
-- category; non-book items carry the canonical source URL they were ingested
-- from (dedup key). RSS subscriptions live in reading.feeds; reading.feed_items
-- is the per-feed seen-set, deliberately independent of library contents so
-- an item the user removed is never re-ingested by the next poll.

-- +goose Up
-- +goose StatementBegin
ALTER TABLE reading.books ADD COLUMN category TEXT NOT NULL DEFAULT 'book';
ALTER TABLE reading.books ADD CONSTRAINT books_category_chk
CHECK (category IN ('book', 'paper', 'article', 'rss'));
ALTER TABLE reading.books ADD COLUMN source_url TEXT;
CREATE UNIQUE INDEX books_source_url_idx ON reading.books (source_url)
WHERE source_url IS NOT NULL;

CREATE TABLE reading.feeds (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id TEXT NOT NULL,
    url TEXT NOT NULL,
    title TEXT NOT NULL DEFAULT '',
    kobo_sync BOOLEAN NOT NULL DEFAULT FALSE,
    etag TEXT,
    last_modified TEXT,
    last_fetched_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, url)
);
CREATE INDEX idx_feeds_user_id ON reading.feeds (user_id);
CREATE TRIGGER trg_feeds_updated_at
BEFORE UPDATE ON reading.feeds
FOR EACH ROW EXECUTE FUNCTION reading.set_updated_at();

CREATE TABLE reading.feed_items (
    feed_id UUID NOT NULL REFERENCES reading.feeds (id) ON DELETE CASCADE,
    guid TEXT NOT NULL,
    book_id UUID REFERENCES reading.books (id) ON DELETE SET NULL,
    ingest_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (feed_id, guid)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE reading.feed_items;
DROP TABLE reading.feeds;
DROP INDEX reading.books_source_url_idx;
ALTER TABLE reading.books DROP COLUMN source_url;
ALTER TABLE reading.books DROP CONSTRAINT books_category_chk;
ALTER TABLE reading.books DROP COLUMN category;
-- +goose StatementEnd
