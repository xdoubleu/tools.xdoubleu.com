-- +goose Up
-- +goose StatementBegin
-- Task numbers a job found its task waits on; it resumes once they merge.
-- New column, nothing to backfill: tasks parked before it stay manual.
ALTER TABLE factory.tasks
ADD COLUMN waiting_on bigint[] NOT NULL DEFAULT '{}';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE factory.tasks DROP COLUMN IF EXISTS waiting_on;
-- +goose StatementEnd
