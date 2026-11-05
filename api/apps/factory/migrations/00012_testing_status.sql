-- +goose Up
-- +goose StatementBegin
-- A new column for merged work awaiting a person's check. No backfill: existing
-- rows keep their meaning.
ALTER TABLE factory.tasks DROP CONSTRAINT IF EXISTS tasks_status_check;
ALTER TABLE factory.tasks
ADD CONSTRAINT tasks_status_check CHECK (
    status IN (
        'backlog', 'ready', 'in_progress', 'in_review', 'needs_you', 'testing',
        'done'
    )
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE factory.tasks SET status = 'done'
WHERE status = 'testing';
ALTER TABLE factory.tasks DROP CONSTRAINT IF EXISTS tasks_status_check;
ALTER TABLE factory.tasks
ADD CONSTRAINT tasks_status_check CHECK (
    status IN (
        'backlog', 'ready', 'in_progress', 'in_review', 'needs_you', 'done'
    )
);
-- +goose StatementEnd
