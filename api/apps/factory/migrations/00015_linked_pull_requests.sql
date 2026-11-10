-- +goose Up
-- +goose StatementBegin
-- Pull requests that link a task in their body (`Task: <title> (<id>)`),
-- whoever opened them, as the poller last read them. A new table, so no
-- backfill.
CREATE TABLE IF NOT EXISTS factory.linked_pull_requests (
    url TEXT PRIMARY KEY,
    task_id UUID NOT NULL REFERENCES factory.tasks (id) ON DELETE CASCADE,
    state TEXT NOT NULL,
    ci TEXT NOT NULL DEFAULT '',
    review TEXT NOT NULL DEFAULT '',
    checked_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS linked_pull_requests_task_idx
ON factory.linked_pull_requests (task_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS factory.linked_pull_requests;
-- +goose StatementEnd
