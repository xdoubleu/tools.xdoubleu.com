-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS backlog.book_reading_state (
    user_id TEXT NOT NULL,
    book_id UUID NOT NULL REFERENCES backlog.books (id) ON DELETE CASCADE,
    source TEXT NOT NULL CHECK (source IN ('web', 'kobo', 'manual')),
    percent SMALLINT NOT NULL DEFAULT 0 CHECK (percent BETWEEN 0 AND 100),
    location TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, book_id)
);

DROP TRIGGER IF EXISTS trg_reading_state_upd_at ON backlog.book_reading_state;
CREATE TRIGGER trg_reading_state_upd_at
BEFORE UPDATE ON backlog.book_reading_state
FOR EACH ROW EXECUTE FUNCTION backlog.set_updated_at();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_reading_state_upd_at ON backlog.book_reading_state;
DROP TABLE IF EXISTS backlog.book_reading_state;
-- +goose StatementEnd
