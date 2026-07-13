-- +goose Up
-- +goose StatementBegin
ALTER TABLE books.books DROP COLUMN googlebooks_found;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE books.books ADD COLUMN googlebooks_found BOOLEAN;
-- +goose StatementEnd
