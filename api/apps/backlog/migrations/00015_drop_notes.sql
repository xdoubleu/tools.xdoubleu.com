-- +goose Up
ALTER TABLE backlog.user_books DROP COLUMN notes;

-- +goose Down
ALTER TABLE backlog.user_books ADD COLUMN notes TEXT;
