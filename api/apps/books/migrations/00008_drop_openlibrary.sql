-- +goose Up
-- +goose StatementBegin
ALTER TABLE books.books DROP COLUMN openlibrary_found;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE books.books ADD COLUMN openlibrary_found BOOLEAN;
-- +goose StatementEnd
