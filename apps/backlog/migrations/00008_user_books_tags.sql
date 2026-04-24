-- +goose Up
-- +goose StatementBegin

ALTER TABLE backlog.user_books
ALTER COLUMN tags DROP NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- +goose StatementEnd
