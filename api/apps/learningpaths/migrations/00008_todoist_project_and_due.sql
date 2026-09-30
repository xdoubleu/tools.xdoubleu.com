-- +goose Up
-- +goose StatementBegin
ALTER TABLE learningpaths.learning_paths
ADD COLUMN IF NOT EXISTS todoist_project_id TEXT;

-- due is a Todoist natural-language due string; '' leaves the task undated.
ALTER TABLE learningpaths.items
ADD COLUMN IF NOT EXISTS due TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE learningpaths.items
DROP COLUMN IF EXISTS due;

ALTER TABLE learningpaths.learning_paths
DROP COLUMN IF EXISTS todoist_project_id;
-- +goose StatementEnd
