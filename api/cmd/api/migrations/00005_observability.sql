-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS global.job_runs (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    job_id TEXT NOT NULL,
    started_at TIMESTAMPTZ NOT NULL,
    duration_ms BIGINT NOT NULL,
    success BOOLEAN NOT NULL,
    error TEXT
);
CREATE INDEX IF NOT EXISTS idx_job_runs_job_id_started_at
ON global.job_runs (job_id, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_job_runs_started_at
ON global.job_runs (started_at);

CREATE TABLE IF NOT EXISTS global.usage_daily (
    day DATE NOT NULL,
    app TEXT NOT NULL,
    endpoint TEXT NOT NULL,
    count BIGINT NOT NULL,
    PRIMARY KEY (day, app, endpoint)
);

CREATE TABLE IF NOT EXISTS global.storage_snapshots (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    scanned_at TIMESTAMPTZ NOT NULL,
    total_size_bytes BIGINT NOT NULL,
    object_count BIGINT NOT NULL,
    orphan_size_bytes BIGINT NOT NULL,
    orphan_count BIGINT NOT NULL,
    stale_upload_size_bytes BIGINT NOT NULL,
    stale_upload_count BIGINT NOT NULL,
    prefix_breakdown JSONB NOT NULL
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS global.storage_snapshots;
DROP TABLE IF EXISTS global.usage_daily;
DROP TABLE IF EXISTS global.job_runs;
-- +goose StatementEnd
