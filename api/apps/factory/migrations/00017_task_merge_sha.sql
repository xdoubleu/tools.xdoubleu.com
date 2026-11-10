-- +goose Up
-- +goose StatementBegin
-- The commit a task's pull request merged as, to tell if it is deployed. No
-- backfill: tasks merged before this keep no deploy state.
ALTER TABLE factory.tasks ADD COLUMN merge_sha text NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE factory.tasks DROP COLUMN merge_sha;
-- +goose StatementEnd
