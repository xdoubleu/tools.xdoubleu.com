-- +goose Up
ALTER TABLE backlog.user_books
ADD COLUMN IF NOT EXISTS shelf_positions JSONB NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE backlog.user_books
DROP COLUMN IF EXISTS shelf_positions;
