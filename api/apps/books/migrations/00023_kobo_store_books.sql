-- +goose Up
-- +goose StatementBegin
-- Kobo store (bol.com) books seen through a user's Kobo, keyed by the store's
-- entitlement id. Matched to library books at use time, so a book added to the
-- library later still links. owned is learned from the store entitlement.
CREATE TABLE books.kobo_store_books (
    user_id TEXT NOT NULL,
    entitlement_id TEXT NOT NULL,
    isbn13 TEXT,
    title TEXT NOT NULL DEFAULT '',
    authors TEXT[] NOT NULL DEFAULT '{}',
    owned BOOLEAN NOT NULL DEFAULT false,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, entitlement_id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE books.kobo_store_books;
-- +goose StatementEnd
