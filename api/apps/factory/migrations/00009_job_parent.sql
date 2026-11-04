-- +goose Up
-- +goose StatementBegin
-- The job a Reply resumes: the new job continues that job's harness session on
-- its branch. Null for a job started by Run. New column, so no backfill.
ALTER TABLE factory.jobs
ADD COLUMN IF NOT EXISTS parent_job_id UUID
REFERENCES factory.jobs (id) ON DELETE SET NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE factory.jobs DROP COLUMN IF EXISTS parent_job_id;
-- +goose StatementEnd
