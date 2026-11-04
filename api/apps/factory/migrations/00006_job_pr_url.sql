-- +goose Up
-- +goose StatementBegin
-- The pull request a job opened. Empty for every job before this and for any
-- that opened none, so no backfill.
ALTER TABLE factory.jobs
ADD COLUMN IF NOT EXISTS pr_url TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE factory.jobs DROP COLUMN IF EXISTS pr_url;
-- +goose StatementEnd
