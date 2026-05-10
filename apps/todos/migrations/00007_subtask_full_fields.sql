-- +goose Up
-- +goose StatementBegin
ALTER TABLE todos.subtasks
ADD COLUMN IF NOT EXISTS priority INT NOT NULL DEFAULT 0,
ADD COLUMN IF NOT EXISTS label TEXT NOT NULL DEFAULT '',
ADD COLUMN IF NOT EXISTS due_date DATE,
ADD COLUMN IF NOT EXISTS deadline DATE;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE todos.subtasks
DROP COLUMN IF EXISTS priority,
DROP COLUMN IF EXISTS label,
DROP COLUMN IF EXISTS due_date,
DROP COLUMN IF EXISTS deadline;
-- +goose StatementEnd
