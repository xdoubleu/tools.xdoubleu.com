-- +goose Up
-- +goose StatementBegin

ALTER SCHEMA goaltracker RENAME TO backlog;
ALTER TABLE backlog.user_books
ALTER COLUMN tags DROP NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- +goose StatementEnd
