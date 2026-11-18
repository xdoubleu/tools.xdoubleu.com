-- +goose Up
-- +goose StatementBegin
-- One shared Todoist project per user; each path is a section in it. The
-- per-path todoist_project_id now only names a legacy project the next sync
-- deletes, so no backfill is needed.
CREATE TABLE IF NOT EXISTS learningpaths.todoist_projects (
    user_id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL
);

ALTER TABLE learningpaths.learning_paths
ADD COLUMN IF NOT EXISTS todoist_section_id TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE learningpaths.learning_paths
DROP COLUMN IF EXISTS todoist_section_id;

DROP TABLE IF EXISTS learningpaths.todoist_projects;
-- +goose StatementEnd
