-- +goose Up
-- +goose StatementBegin
-- The harness's stream-json events as the runner forwarded them, secrets
-- already removed. Deleted with their job, and by a retention job.
CREATE TABLE IF NOT EXISTS factory.job_events (
    job_id UUID NOT NULL REFERENCES factory.jobs (id) ON DELETE CASCADE,
    seq INTEGER NOT NULL,
    kind TEXT NOT NULL,
    payload TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (job_id, seq)
);

CREATE INDEX IF NOT EXISTS idx_job_events_created
ON factory.job_events (created_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS factory.job_events;
-- +goose StatementEnd
