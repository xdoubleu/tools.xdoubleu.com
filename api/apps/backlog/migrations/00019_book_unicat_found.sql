-- +goose Up
ALTER TABLE backlog.books
ADD COLUMN IF NOT EXISTS unicat_found BOOLEAN;

-- +goose Down
ALTER TABLE backlog.books
DROP COLUMN IF EXISTS unicat_found;
