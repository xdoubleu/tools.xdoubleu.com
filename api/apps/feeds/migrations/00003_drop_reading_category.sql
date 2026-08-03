-- Issue #735: reading.books.category (book/paper/article/rss) is retired.
-- This must run as a feeds migration, not a reading one: feeds registers
-- after reading (see cmd/api/apps.go), so a reading-side migration dropping
-- the column would always run before 00002_migrate_reading_feed_data.sql's
-- `category = 'rss'` cleanup gets a chance to run on a fresh install,
-- breaking that migration. Guarded the same way as 00002: a no-op when
-- reading.books/category doesn't exist (fresh install past this point, or
-- this app's own isolated test suite, which never runs reading's migrations).

-- +goose Up
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'reading' AND table_name = 'books'
            AND column_name = 'category'
    ) THEN
        DELETE FROM reading.books WHERE category = 'rss';
        ALTER TABLE reading.books DROP CONSTRAINT books_category_chk;
        ALTER TABLE reading.books DROP COLUMN category;
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- Not reversible: see 00002's Down for the same reasoning. Best effort:
-- recreate the column so a subsequent Up can run again.
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_schema = 'reading' AND table_name = 'books'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'reading' AND table_name = 'books'
            AND column_name = 'category'
    ) THEN
        ALTER TABLE reading.books ADD COLUMN category TEXT NOT NULL DEFAULT 'book';
        ALTER TABLE reading.books ADD CONSTRAINT books_category_chk
        CHECK (category IN ('book', 'paper', 'article', 'rss'));
    END IF;
END $$;
-- +goose StatementEnd
