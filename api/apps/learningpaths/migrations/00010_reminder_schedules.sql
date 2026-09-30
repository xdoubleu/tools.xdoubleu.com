-- +goose Up
-- +goose StatementBegin
-- reminder_schedules maps an item type to a Todoist due string; '{}' leaves
-- tasks without their own due undated, as before.
ALTER TABLE learningpaths.learning_paths
ADD COLUMN IF NOT EXISTS reminder_schedules JSONB NOT NULL DEFAULT '{}';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE learningpaths.learning_paths
DROP COLUMN IF EXISTS reminder_schedules;
-- +goose StatementEnd
