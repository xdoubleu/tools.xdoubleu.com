-- Adopt the book tables from the former backlog schema when it exists
-- (production upgrade path), or create them fresh (new installations).
-- The books schema itself is created by ApplyMigrationsFromFS before goose
-- runs. Shapes match the final backlog state (migrations 00001-00019).
-- ALTER TABLE ... SET SCHEMA moves indexes, constraints, and triggers, but
-- the moved triggers still reference backlog.set_updated_at(); they are
-- re-pointed at reading.set_updated_at() below so the backlog schema can be
-- dropped.

-- +goose Up
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_schema = 'backlog' AND table_name = 'books'
    ) THEN
        ALTER TABLE backlog.books SET SCHEMA reading;
        ALTER TABLE backlog.user_books SET SCHEMA reading;
        ALTER TABLE backlog.book_files SET SCHEMA reading;
        ALTER TABLE backlog.book_reading_state SET SCHEMA reading;
        ALTER TABLE backlog.kobo_devices SET SCHEMA reading;
    ELSE
        CREATE TABLE reading.books (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            title TEXT NOT NULL,
            authors TEXT [] NOT NULL DEFAULT '{}',
            isbn13 TEXT,
            cover_url TEXT,
            description TEXT,
            page_count INTEGER,
            openlibrary_found BOOLEAN,
            googlebooks_found BOOLEAN,
            unicat_found BOOLEAN,
            last_resync_at TIMESTAMPTZ,
            created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
        );

        CREATE UNIQUE INDEX books_isbn13_idx ON reading.books (
            isbn13
        ) WHERE isbn13 IS NOT NULL;

        CREATE TABLE reading.user_books (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            user_id TEXT NOT NULL,
            book_id UUID NOT NULL REFERENCES reading.books (
                id
            ) ON DELETE CASCADE,
            status TEXT NOT NULL,
            rating SMALLINT,
            finished_at TIMESTAMPTZ [],
            tags TEXT [] NOT NULL DEFAULT '{}',
            shelf_positions JSONB NOT NULL DEFAULT '{}',
            progress_mode TEXT NOT NULL DEFAULT 'pages',
            current_page INTEGER NOT NULL DEFAULT 0,
            progress_percent SMALLINT NOT NULL DEFAULT 0,
            kobo_sync_enabled_at TIMESTAMPTZ,
            added_at TIMESTAMPTZ NOT NULL DEFAULT now(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
            UNIQUE (user_id, book_id),
            CONSTRAINT chk_user_books_rating CHECK (
                rating IS NULL OR rating BETWEEN 1 AND 5
            ),
            CONSTRAINT user_books_progress_mode_chk CHECK (
                progress_mode IN ('pages', 'percent')
            ),
            CONSTRAINT user_books_progress_percent_chk CHECK (
                progress_percent BETWEEN 0 AND 100
            )
        );

        CREATE INDEX idx_user_books_user_id ON reading.user_books (user_id);
        CREATE INDEX idx_user_books_status ON reading.user_books (status);

        CREATE TABLE reading.book_files (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            book_id UUID NOT NULL REFERENCES reading.books (
                id
            ) ON DELETE CASCADE,
            user_id TEXT NOT NULL,
            format TEXT NOT NULL CHECK (format IN ('pdf', 'epub', 'kepub')),
            storage_key TEXT NOT NULL,
            size_bytes BIGINT NOT NULL DEFAULT 0,
            checksum TEXT,
            original_filename TEXT,
            status TEXT NOT NULL DEFAULT 'ready' CHECK (
                status IN ('ready', 'converting', 'failed')
            ),
            source_file_id UUID REFERENCES reading.book_files (
                id
            ) ON DELETE SET NULL,
            created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
        );

        CREATE INDEX idx_book_files_book_id ON reading.book_files (book_id);
        CREATE INDEX idx_book_files_user_id ON reading.book_files (user_id);
        CREATE INDEX idx_book_files_book_format ON reading.book_files (
            book_id, format
        );
        CREATE INDEX idx_book_files_checksum ON reading.book_files (checksum);
        CREATE INDEX idx_book_files_storage_key ON reading.book_files (
            storage_key
        );

        CREATE TABLE reading.book_reading_state (
            user_id TEXT NOT NULL,
            book_id UUID NOT NULL REFERENCES reading.books (
                id
            ) ON DELETE CASCADE,
            source TEXT NOT NULL CHECK (source IN ('web', 'kobo', 'manual')),
            percent SMALLINT NOT NULL DEFAULT 0 CHECK (
                percent BETWEEN 0 AND 100
            ),
            location TEXT,
            updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
            PRIMARY KEY (user_id, book_id)
        );

        CREATE TABLE reading.kobo_devices (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            user_id TEXT NOT NULL,
            token_hash TEXT NOT NULL UNIQUE,
            name TEXT NOT NULL,
            serial TEXT,
            created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
            last_seen_at TIMESTAMPTZ
        );

        CREATE INDEX idx_kobo_devices_user_id ON reading.kobo_devices (user_id);
    END IF;
END $$;
-- +goose StatementEnd

-- Re-point the updated_at triggers at a books-schema function so nothing
-- references backlog.set_updated_at() once the backlog schema is dropped.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION reading.set_updated_at()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN NEW.updated_at = now(); RETURN NEW; END;
$$;

DROP TRIGGER IF EXISTS trg_user_books_updated_at ON reading.user_books;
CREATE TRIGGER trg_user_books_updated_at
BEFORE UPDATE ON reading.user_books
FOR EACH ROW EXECUTE FUNCTION reading.set_updated_at();

DROP TRIGGER IF EXISTS trg_book_files_updated_at ON reading.book_files;
CREATE TRIGGER trg_book_files_updated_at
BEFORE UPDATE ON reading.book_files
FOR EACH ROW EXECUTE FUNCTION reading.set_updated_at();

DROP TRIGGER IF EXISTS trg_reading_state_upd_at ON reading.book_reading_state;
CREATE TRIGGER trg_reading_state_upd_at
BEFORE UPDATE ON reading.book_reading_state
FOR EACH ROW EXECUTE FUNCTION reading.set_updated_at();
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS reading.progress (
    user_id TEXT NOT NULL,
    date DATE NOT NULL,
    value VARCHAR NOT NULL,
    PRIMARY KEY (user_id, date)
);
-- +goose StatementEnd

-- Copy the books-read rows (type_id '1') out of the shared backlog table.
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_schema = 'backlog' AND table_name = 'progress'
    ) THEN
        INSERT INTO reading.progress (user_id, date, value)
        SELECT user_id, date, value
        FROM backlog.progress
        WHERE type_id = '1'
        ON CONFLICT DO NOTHING;
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- Not reversible: the backlog schema no longer exists to move tables back to.
-- Restore from a database backup instead.
-- +goose StatementBegin
SELECT 1;
-- +goose StatementEnd
