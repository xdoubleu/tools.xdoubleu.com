-- +goose Up
-- +goose StatementBegin
-- The harness and model that finished a job after it continued on the fallback.
-- Nothing to backfill: earlier jobs were finished by the first harness.
ALTER TABLE factory.jobs
ADD COLUMN finished_harness text NOT NULL DEFAULT '',
ADD COLUMN finished_model text NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE factory.jobs
DROP COLUMN IF EXISTS finished_harness,
DROP COLUMN IF EXISTS finished_model;
-- +goose StatementEnd
