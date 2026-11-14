-- +goose Up
-- +goose StatementBegin
-- A runner may run several jobs at once; its cap is enforced when it claims.
-- idx_jobs_one_active_per_task still gives a task at most one unfinished job.
-- No backfill: it only lifts a restriction.
DROP INDEX IF EXISTS factory.idx_jobs_one_running;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Fails while more than one job is running: stop them first.
CREATE UNIQUE INDEX IF NOT EXISTS idx_jobs_one_running
ON factory.jobs ((TRUE)) WHERE status = 'running';
-- +goose StatementEnd
