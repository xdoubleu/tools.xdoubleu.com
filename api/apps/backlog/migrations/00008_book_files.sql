-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS backlog.book_files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    book_id UUID NOT NULL REFERENCES backlog.books (id) ON DELETE CASCADE,
    user_id TEXT NOT NULL,
    format TEXT NOT NULL CHECK (format IN ('pdf', 'epub', 'kepub')),
    storage_key TEXT NOT NULL,
    size_bytes BIGINT NOT NULL DEFAULT 0,
    checksum TEXT,
    original_filename TEXT,
    status TEXT NOT NULL DEFAULT 'ready' CHECK (
        status IN ('ready', 'converting', 'failed')
    ),
    source_file_id UUID REFERENCES backlog.book_files (id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_book_files_book_id ON backlog.book_files ( -- noqa: PG01,LT05
    book_id
);
CREATE INDEX IF NOT EXISTS idx_book_files_user_id ON backlog.book_files ( -- noqa: PG01,LT05
    user_id
);
CREATE INDEX IF NOT EXISTS idx_book_files_book_format ON backlog.book_files ( -- noqa: PG01,LT05
    book_id, format
);

DROP TRIGGER IF EXISTS trg_book_files_updated_at ON backlog.book_files;
CREATE TRIGGER trg_book_files_updated_at
BEFORE UPDATE ON backlog.book_files
FOR EACH ROW EXECUTE FUNCTION backlog.set_updated_at();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_book_files_updated_at ON backlog.book_files;
DROP TABLE IF EXISTS backlog.book_files;
-- +goose StatementEnd
