-- +goose Up
-- +goose StatementBegin
ALTER TABLE learningpaths.items
ADD COLUMN IF NOT EXISTS todoist_task_id TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE learningpaths.items
DROP COLUMN IF EXISTS todoist_task_id;
-- +goose StatementEnd
