-- +goose Up
-- +goose StatementBegin
-- A job the harness's provider stopped on a usage limit, and when it lifts
-- (null if the harness did not say). New columns, so no backfill: no earlier
-- job was recorded as limited.
ALTER TABLE factory.jobs
ADD COLUMN IF NOT EXISTS limited BOOLEAN NOT NULL DEFAULT false,
ADD COLUMN IF NOT EXISTS limit_resets_at TIMESTAMPTZ;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE factory.jobs
DROP COLUMN IF EXISTS limit_resets_at,
DROP COLUMN IF EXISTS limited;
-- +goose StatementEnd
