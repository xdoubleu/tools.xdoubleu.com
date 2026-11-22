-- +goose Up
-- +goose StatementBegin
-- A chat job is one interactive session pinned to a runner. It starts running,
-- never queues, and holds a runner slot until the session stops. Existing jobs
-- are task jobs, which the default covers.
ALTER TABLE factory.jobs
ADD COLUMN IF NOT EXISTS kind TEXT NOT NULL DEFAULT 'task'
CHECK (kind IN ('task', 'chat'));

-- A chat session between a person and a harness on one runner. task_id is
-- optional context. report holds what the session left behind when it stopped.
-- last_seq is the seq of its newest event.
CREATE TABLE IF NOT EXISTS factory.chat_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    repo_id UUID NOT NULL REFERENCES factory.repos (id),
    task_id UUID REFERENCES factory.tasks (id) ON DELETE SET NULL,
    harness TEXT NOT NULL CHECK (harness IN ('claude_code', 'opencode')),
    runner_id UUID NOT NULL REFERENCES factory.runners (id) ON DELETE CASCADE,
    job_id UUID REFERENCES factory.jobs (id) ON DELETE SET NULL,
    state TEXT NOT NULL DEFAULT 'idle' CHECK (
        state IN ('idle', 'working', 'asking', 'stopped')
    ),
    branch TEXT NOT NULL,
    report TEXT NOT NULL DEFAULT '',
    last_seq INTEGER NOT NULL DEFAULT 0,
    created_by TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    stopped_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_chat_sessions_recent
ON factory.chat_sessions (created_at DESC);

CREATE INDEX IF NOT EXISTS idx_chat_sessions_job
ON factory.chat_sessions (job_id);

-- Append-only: the server numbers events 1, 2, 3 per session, so a client
-- replays from the last seq it saw.
CREATE TABLE IF NOT EXISTS factory.chat_events (
    session_id UUID NOT NULL REFERENCES factory.chat_sessions (id)
    ON DELETE CASCADE,
    seq INTEGER NOT NULL,
    kind TEXT NOT NULL,
    payload TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (session_id, seq)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS factory.chat_events;
DROP TABLE IF EXISTS factory.chat_sessions;
DELETE FROM factory.jobs
WHERE kind = 'chat';
ALTER TABLE factory.jobs DROP COLUMN IF EXISTS kind;
-- +goose StatementEnd
