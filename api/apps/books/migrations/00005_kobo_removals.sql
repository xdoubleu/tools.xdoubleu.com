-- +goose Up
-- +goose StatementBegin
-- Tombstone for books that must be actively removed from a Kobo device.
-- No FK to books.books: RemoveFromLibrary can hard-delete the catalog row
-- via DeleteOrphanedBook right after this tombstone is written, and the
-- whole point of the tombstone is to survive that deletion so the removal
-- still reaches the device on the next sync.
CREATE TABLE books.kobo_removals (
    user_id TEXT NOT NULL,
    book_id UUID NOT NULL,
    removed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, book_id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE books.kobo_removals;
-- +goose StatementEnd
