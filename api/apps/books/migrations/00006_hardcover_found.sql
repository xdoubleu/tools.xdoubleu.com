-- +goose Up
-- +goose StatementBegin
ALTER TABLE books.books ADD COLUMN hardcover_found BOOLEAN;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE books.books DROP COLUMN hardcover_found;
-- +goose StatementEnd
