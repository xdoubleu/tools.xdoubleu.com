-- +goose Up
-- +goose StatementBegin
ALTER TABLE reading.books DROP COLUMN openlibrary_found;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE reading.books ADD COLUMN openlibrary_found BOOLEAN;
-- +goose StatementEnd
