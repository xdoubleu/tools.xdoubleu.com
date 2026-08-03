-- Issue #734: feeds/email-newsletter subscriptions move out of
-- reading.feeds/reading.feed_items into this app's own schema. Items become
-- standalone rows here instead of reading.books/reading.user_books entries —
-- their read/favourite state is copied onto the new columns directly since a
-- feed and its items only ever belong to one user. Rows whose feed_items
-- link never produced a book (a total ingest failure) are not migrated; they
-- carried no content anyway.
--
-- Guarded on reading.feeds existing: a fresh install (or this app's own
-- isolated test suite, which never runs reading's migrations) has nothing to
-- copy, so the whole block is a no-op there.

-- +goose Up
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_schema = 'reading' AND table_name = 'feeds'
    ) THEN
        INSERT INTO feeds.feeds
            (id, user_id, url, title, source_type, inbound_token, etag,
             last_modified, last_fetched_at, last_error, created_at,
             updated_at)
        SELECT
            id, user_id, url, title, source_type, inbound_token, etag,
            last_modified, last_fetched_at, last_error, created_at,
            updated_at
        FROM reading.feeds
        ON CONFLICT (user_id, url) DO NOTHING;

        INSERT INTO feeds.items
            (feed_id, guid, title, source_url, content_html, published_at,
             read_at, dismissed, favourite, ingest_error, created_at)
        SELECT
            fi.feed_id,
            fi.guid,
            COALESCE(b.title, ''),
            COALESCE(b.source_url, ''),
            COALESCE(b.content_html, ''),
            COALESCE(ub.added_at, fi.created_at),
            CASE WHEN ub.status = 'read' THEN ub.updated_at END,
            FALSE,
            COALESCE('favourite' = ANY (ub.tags), FALSE),
            fi.ingest_error,
            fi.created_at
        FROM reading.feed_items AS fi
        INNER JOIN reading.books AS b ON b.id = fi.book_id
        LEFT JOIN reading.user_books AS ub ON ub.book_id = b.id
        WHERE fi.book_id IS NOT NULL
        ON CONFLICT (feed_id, guid) DO NOTHING;

        -- Remove the migrated rows from reading: user_books links first
        -- (FK), then the rss-category books, then the now-empty feed
        -- tables. The books.category CHECK constraint still allows 'rss' —
        -- cleaning that up is out of scope.
        DELETE FROM reading.user_books
        WHERE book_id IN (SELECT id FROM reading.books WHERE category = 'rss');
        DELETE FROM reading.books WHERE category = 'rss';
        DROP TABLE reading.feed_items;
        DROP TABLE reading.feeds;
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- Not reversible: this is a one-way data migration off reading's schema, and
-- reading.feeds/reading.feed_items no longer exist by the time Down could
-- run. Best effort: recreate the (empty) tables so a subsequent Up can run
-- again; the migrated feed/item data itself is NOT restored — restore from a
-- database backup if that is needed.
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.schemata WHERE schema_name = 'reading'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_schema = 'reading' AND table_name = 'feeds'
    ) THEN
        CREATE TABLE reading.feeds (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            user_id TEXT NOT NULL,
            url TEXT,
            title TEXT NOT NULL DEFAULT '',
            source_type TEXT NOT NULL DEFAULT 'rss',
            inbound_token TEXT,
            etag TEXT,
            last_modified TEXT,
            last_fetched_at TIMESTAMPTZ,
            last_error TEXT,
            created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
            UNIQUE (user_id, url)
        );
        CREATE TABLE reading.feed_items (
            feed_id UUID NOT NULL REFERENCES reading.feeds (id) ON DELETE CASCADE,
            guid TEXT NOT NULL,
            book_id UUID REFERENCES reading.books (id) ON DELETE SET NULL,
            ingest_error TEXT,
            created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
            PRIMARY KEY (feed_id, guid)
        );
    END IF;
END $$;
-- +goose StatementEnd
