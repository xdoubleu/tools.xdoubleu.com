-- +goose Up
-- +goose StatementBegin
-- Pending KEPUB conversions, drained by the workers role. New table, so no
-- backfill: conversions are re-requested by the next status poll or sync.
CREATE TABLE books.kepub_conversion_requests (
    user_id TEXT NOT NULL,
    book_id UUID NOT NULL REFERENCES books.books (id) ON DELETE CASCADE,
    requested_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, book_id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE books.kepub_conversion_requests;
-- +goose StatementEnd
