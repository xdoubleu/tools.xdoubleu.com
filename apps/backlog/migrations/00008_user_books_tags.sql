-- +goose Up
-- +goose StatementBegin

ALTER TABLE goaltracker.user_books
ALTER COLUMN tags DROP NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- +goose StatementEnd
