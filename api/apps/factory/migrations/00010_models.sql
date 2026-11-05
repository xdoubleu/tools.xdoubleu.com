-- +goose Up
-- +goose StatementBegin
-- The model an OpenCode job runs on (an OpenRouter id such as "vendor/model"), as a
-- repo's default and as frozen on each job. Claude Code picks its own model, so it
-- stays empty there. New columns, so no backfill: existing repos and jobs are
-- Claude Code with no model.
ALTER TABLE factory.repos
ADD COLUMN IF NOT EXISTS default_model TEXT NOT NULL DEFAULT '';
ALTER TABLE factory.jobs
ADD COLUMN IF NOT EXISTS model TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE factory.jobs DROP COLUMN IF EXISTS model;
ALTER TABLE factory.repos DROP COLUMN IF EXISTS default_model;
-- +goose StatementEnd
