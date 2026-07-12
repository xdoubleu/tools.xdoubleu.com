-- +goose Up
-- +goose StatementBegin
CREATE TABLE books.shelves (
    user_id TEXT NOT NULL,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, name)
);
-- +goose StatementEnd

-- +goose StatementBegin
-- Backfill: register every custom status already in use so existing custom
-- shelves keep existing once this change starts hiding empty shelves from the
-- ad-hoc user_books derivation.
INSERT INTO books.shelves (user_id, name)
SELECT DISTINCT
    user_id,
    status
FROM books.user_books
WHERE status NOT IN ('to-read', 'currently-reading', 'read', 'dropped')
ON CONFLICT DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE books.shelves;
-- +goose StatementEnd
