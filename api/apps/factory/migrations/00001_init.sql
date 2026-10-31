-- +goose Up
-- +goose StatementBegin
CREATE SCHEMA IF NOT EXISTS factory;

CREATE TABLE IF NOT EXISTS factory.tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title TEXT NOT NULL,
    body TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'backlog' CHECK (
        status IN (
            'backlog', 'ready', 'in_progress', 'in_review', 'needs_you', 'done'
        )
    ),
    priority TEXT NOT NULL DEFAULT 'p2' CHECK (priority IN ('p0', 'p1', 'p2')),
    labels TEXT[] NOT NULL DEFAULT '{}',
    -- Actors as "<kind>:<id>".
    created_by TEXT NOT NULL,
    body_edited_by TEXT NOT NULL,
    -- How the last body edit was authenticated (browser or mcp).
    body_edited_channel TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_tasks_status_updated
ON factory.tasks (
    status, updated_at DESC
);

CREATE TABLE IF NOT EXISTS factory.task_comments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES factory.tasks (id) ON DELETE CASCADE,
    author TEXT NOT NULL,
    body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_task_comments_task
ON factory.task_comments (
    task_id, created_at
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP SCHEMA IF EXISTS factory CASCADE;
-- +goose StatementEnd
