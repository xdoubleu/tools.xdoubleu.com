-- +goose Up
-- +goose StatementBegin
-- Where a job's pull request stands, as the poller last read it from GitHub.
-- Empty until the first read, so no backfill.
ALTER TABLE factory.jobs
ADD COLUMN IF NOT EXISTS pr_state TEXT NOT NULL DEFAULT ''
CHECK (pr_state IN ('', 'open', 'merged', 'closed')),
ADD COLUMN IF NOT EXISTS pr_ci TEXT NOT NULL DEFAULT ''
CHECK (pr_ci IN ('', 'pending', 'passing', 'failing')),
ADD COLUMN IF NOT EXISTS pr_review TEXT NOT NULL DEFAULT ''
CHECK (pr_review IN ('', 'approved', 'changes_requested')),
ADD COLUMN IF NOT EXISTS pr_checked_at TIMESTAMPTZ;

-- The poller's worklist: pull requests that are not over.
CREATE INDEX IF NOT EXISTS idx_jobs_open_pr
ON factory.jobs (pr_checked_at NULLS FIRST)
WHERE pr_url <> '' AND pr_state NOT IN ('merged', 'closed');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS factory.idx_jobs_open_pr;
ALTER TABLE factory.jobs
DROP COLUMN IF EXISTS pr_checked_at,
DROP COLUMN IF EXISTS pr_review,
DROP COLUMN IF EXISTS pr_ci,
DROP COLUMN IF EXISTS pr_state;
-- +goose StatementEnd
