-- +goose Up
-- +goose StatementBegin
-- kobo_location is the full Kobo bookmark {Source, Type, Value}; location
-- keeps the bare span value. read_at is the reading device's own timestamp.
-- Both are new and nullable, so existing rows need no backfill.
ALTER TABLE books.book_reading_state
ADD COLUMN kobo_location JSONB,
ADD COLUMN read_at TIMESTAMPTZ;

-- The book_reading_state.updated_at each Kobo device last holds, sent to it
-- or received from it. A row also means the book is on that device.
CREATE TABLE books.kobo_device_reading_states (
    device_id UUID NOT NULL REFERENCES books.kobo_devices (id)
    ON DELETE CASCADE,
    book_id UUID NOT NULL REFERENCES books.books (id) ON DELETE CASCADE,
    state_updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (device_id, book_id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE books.kobo_device_reading_states;

ALTER TABLE books.book_reading_state
DROP COLUMN read_at,
DROP COLUMN kobo_location;
-- +goose StatementEnd
