-- +goose Up
-- +goose StatementBegin
ALTER TABLE todos.subtasks
ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE todos.subtasks DROP COLUMN IF EXISTS description;
-- +goose StatementEnd
