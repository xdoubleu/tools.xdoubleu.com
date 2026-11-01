-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS factory.jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES factory.tasks (id) ON DELETE CASCADE,
    repo_id UUID NOT NULL REFERENCES factory.repos (id),
    harness TEXT NOT NULL CHECK (harness IN ('claude_code', 'opencode')),
    -- Frozen when Run is pressed: repo instructions, the task as last edited in
    -- a browser, and the Run text. The runner adds its own preamble.
    prompt TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'queued' CHECK (
        status IN (
            'queued', 'running', 'succeeded', 'failed', 'cancelled', 'lost'
        )
    ),
    branch TEXT NOT NULL,
    -- The harness's session id, kept for Reply.
    session_id TEXT NOT NULL DEFAULT '',
    outcome TEXT NOT NULL DEFAULT '',
    created_by TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at TIMESTAMPTZ,
    last_check_in TIMESTAMPTZ,
    finished_at TIMESTAMPTZ
);

-- FIFO: the oldest queued job is claimed first.
CREATE INDEX IF NOT EXISTS idx_jobs_queue
ON factory.jobs (created_at) WHERE status = 'queued';

-- One job at a time: a second running row conflicts.
CREATE UNIQUE INDEX IF NOT EXISTS idx_jobs_one_running
ON factory.jobs ((TRUE)) WHERE status = 'running';

CREATE INDEX IF NOT EXISTS idx_jobs_task
ON factory.jobs (task_id, created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS factory.jobs;
-- +goose StatementEnd
