-- +goose Up
-- +goose StatementBegin
ALTER TABLE reading.books ADD COLUMN metadata_source TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE reading.books DROP COLUMN metadata_source;
-- +goose StatementEnd
