-- +goose Up
-- +goose StatementBegin
-- A routine is a named, scheduled job with no task. Only a human admin in a
-- browser changes a row (adr-0052); its prompt lives in the repo, not here.
CREATE TABLE IF NOT EXISTS factory.routines (
    name TEXT PRIMARY KEY CHECK (name ~ '^[a-z][a-z0-9-]{0,39}$'),
    repo_id UUID NOT NULL REFERENCES factory.repos (id),
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    -- Minutes between scheduled runs; 0 means manual runs only.
    interval_minutes INT NOT NULL DEFAULT 0 CHECK (interval_minutes >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- The no-op routine proves the path end to end: manual runs only, off until a
-- human enables it.
INSERT INTO factory.routines (name, repo_id)
SELECT
    'noop',
    id
FROM factory.repos
WHERE owner = 'xdoubleu' AND name = 'tools.xdoubleu.com'
ON CONFLICT (name) DO NOTHING;

-- A routine's job has no task. No backfill: every existing job has a task.
ALTER TABLE factory.jobs ALTER COLUMN task_id DROP NOT NULL;
ALTER TABLE factory.jobs
ADD COLUMN IF NOT EXISTS routine TEXT NOT NULL DEFAULT '';
ALTER TABLE factory.jobs
ADD CONSTRAINT jobs_task_xor_routine CHECK (
    (task_id IS NULL) = (routine <> '')
);

-- A routine has at most one unfinished job, so missed runs coalesce.
CREATE UNIQUE INDEX IF NOT EXISTS idx_jobs_one_active_per_routine
ON factory.jobs (routine)
WHERE routine <> '' AND status IN ('queued', 'running');

CREATE INDEX IF NOT EXISTS idx_jobs_routine
ON factory.jobs (routine, created_at DESC) WHERE routine <> '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS factory.idx_jobs_routine;
DROP INDEX IF EXISTS factory.idx_jobs_one_active_per_routine;
DELETE FROM factory.jobs WHERE routine <> '';
ALTER TABLE factory.jobs DROP CONSTRAINT IF EXISTS jobs_task_xor_routine;
ALTER TABLE factory.jobs DROP COLUMN IF EXISTS routine;
ALTER TABLE factory.jobs ALTER COLUMN task_id SET NOT NULL;
DROP TABLE IF EXISTS factory.routines;
-- +goose StatementEnd
