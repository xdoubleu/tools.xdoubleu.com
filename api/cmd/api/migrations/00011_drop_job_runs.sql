-- +goose Up
-- +goose StatementBegin
DROP TABLE IF EXISTS global.job_runs;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS global.job_runs (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    job_id TEXT NOT NULL,
    started_at TIMESTAMPTZ NOT NULL,
    duration_ms BIGINT NOT NULL,
    success BOOLEAN NOT NULL,
    error TEXT
);
CREATE INDEX idx_job_runs_job_id_started_at ON global.job_runs (
    job_id, started_at DESC
);
CREATE INDEX idx_job_runs_started_at ON global.job_runs (started_at);
-- +goose StatementEnd
