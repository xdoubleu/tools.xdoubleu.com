-- +goose Up
-- +goose StatementBegin
ALTER TABLE reading.books ADD COLUMN hardcover_found BOOLEAN;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE reading.books DROP COLUMN hardcover_found;
-- +goose StatementEnd
