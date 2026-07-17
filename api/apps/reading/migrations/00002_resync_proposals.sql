-- +goose Up
-- +goose StatementBegin
CREATE TABLE reading.resync_proposals (
    book_id UUID PRIMARY KEY REFERENCES reading.books (id) ON DELETE CASCADE,
    proposals JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE reading.resync_proposals;
-- +goose StatementEnd
