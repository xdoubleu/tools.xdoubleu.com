-- +goose Up
-- +goose StatementBegin
CREATE TABLE books.resync_proposals (
    book_id UUID PRIMARY KEY REFERENCES books.books (id) ON DELETE CASCADE,
    proposals JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE books.resync_proposals;
-- +goose StatementEnd
