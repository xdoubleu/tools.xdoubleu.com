-- +goose Up
-- +goose StatementBegin
ALTER TABLE books.books ADD COLUMN metadata_source TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE books.books DROP COLUMN metadata_source;
-- +goose StatementEnd
