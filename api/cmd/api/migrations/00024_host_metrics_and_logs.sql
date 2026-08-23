-- +goose Up
-- +goose StatementBegin
-- Host CPU/memory/disk time series, scraped from node_exporter's Prometheus
-- text-exposition format by internal/observability/jobs.HostMetricsSnapshotJob
-- (issue #1040) — no standalone Prometheus server.
CREATE TABLE global.host_metric_samples (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    sampled_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    cpu_percent DOUBLE PRECISION NOT NULL,
    memory_percent DOUBLE PRECISION NOT NULL,
    disk_percent DOUBLE PRECISION NOT NULL
);

CREATE INDEX host_metric_samples_sampled_at_idx
ON global.host_metric_samples (sampled_at);

-- Application logs forwarded from both api (in-process) and web (HTTP
-- ingest), so log history lives in the DB instead of only in container
-- stdout / Sentry.
CREATE TABLE global.log_entries (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    source TEXT NOT NULL CHECK (source IN ('api', 'web')),
    level TEXT NOT NULL,
    message TEXT NOT NULL,
    attrs JSONB
);

CREATE INDEX log_entries_occurred_at_idx ON global.log_entries (occurred_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS global.log_entries;
DROP TABLE IF EXISTS global.host_metric_samples;
-- +goose StatementEnd
