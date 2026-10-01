-- +goose Up
-- +goose StatementBegin
-- paused stops Todoist reminders for a path; existing paths stay active.
ALTER TABLE learningpaths.learning_paths
ADD COLUMN IF NOT EXISTS paused BOOLEAN NOT NULL DEFAULT false;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE learningpaths.learning_paths
DROP COLUMN IF EXISTS paused;
-- +goose StatementEnd
