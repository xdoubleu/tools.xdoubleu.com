-- +goose Up
-- +goose StatementBegin
-- Issue #1468: Prometheus + Grafana replace the hand-rolled host/CI/storage
-- metrics pipeline. host_metric_samples (HostMetricsSnapshotJob),
-- db_size_samples (DBSizeSnapshotJob), and workflow_run_samples/
-- workflow_job_samples (WorkflowRunsSnapshotJob) are no longer written —
-- Prometheus scrapes node_exporter/postgres_exporter/api's own /metrics
-- directly instead, and Grafana graphs it. global.transaction_latency_daily
-- is NOT dropped here: WeeklyDigestJob and GetSlowTransactions' trending
-- section still read it (it's Sentry-derived, not host-metrics-derived, and
-- ADR-0011's slow-transaction thresholds are explicitly kept).
DROP TABLE IF EXISTS global.host_metric_samples;
DROP TABLE IF EXISTS global.db_size_samples;
DROP TABLE IF EXISTS global.workflow_job_samples;
DROP TABLE IF EXISTS global.workflow_run_samples;

-- The host/CI/storage threshold rules move to Grafana alert rules
-- (infra/prometheus/alert-rules.yml); their toggles and any stored state no
-- longer apply. slow_transaction_*/other sources are untouched.
DELETE FROM global.notification_settings
WHERE source_key IN (
    'host_cpu_high', 'host_memory_high', 'host_disk_high',
    'r2_usage_high', 'ci_duration_high', 'failing_main_ci'
);
DELETE FROM global.alert_states
WHERE rule_key IN (
    'host_cpu_high', 'host_memory_high', 'host_disk_high',
    'r2_usage_high', 'ci_duration_high'
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
INSERT INTO global.notification_settings (source_key) VALUES
('host_cpu_high'),
('host_memory_high'),
('host_disk_high'),
('r2_usage_high'),
('ci_duration_high'),
('failing_main_ci')
ON CONFLICT (source_key) DO NOTHING;

CREATE TABLE IF NOT EXISTS global.workflow_run_samples (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    run_id BIGINT NOT NULL UNIQUE,
    workflow_name TEXT NOT NULL,
    branch TEXT NOT NULL,
    event TEXT NOT NULL,
    conclusion TEXT NOT NULL,
    url TEXT NOT NULL,
    duration_ms BIGINT NOT NULL,
    started_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS workflow_run_samples_branch_conclusion_idx
ON global.workflow_run_samples (branch, conclusion);
CREATE INDEX IF NOT EXISTS workflow_run_samples_started_at_idx
ON global.workflow_run_samples (started_at);

CREATE TABLE IF NOT EXISTS global.workflow_job_samples (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    run_id BIGINT NOT NULL,
    job_name TEXT NOT NULL,
    conclusion TEXT NOT NULL,
    duration_ms BIGINT NOT NULL,
    started_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS workflow_job_samples_run_id_idx
ON global.workflow_job_samples (run_id);
CREATE INDEX IF NOT EXISTS workflow_job_samples_job_name_idx
ON global.workflow_job_samples (job_name);

CREATE TABLE IF NOT EXISTS global.db_size_samples (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    sampled_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    schema_name TEXT NOT NULL,
    table_name TEXT NOT NULL,
    size_bytes BIGINT NOT NULL
);
CREATE INDEX IF NOT EXISTS db_size_samples_sampled_at_idx
ON global.db_size_samples (sampled_at);

CREATE TABLE IF NOT EXISTS global.host_metric_samples (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    sampled_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    cpu_percent DOUBLE PRECISION NOT NULL,
    memory_percent DOUBLE PRECISION NOT NULL,
    disk_percent DOUBLE PRECISION NOT NULL
);
CREATE INDEX IF NOT EXISTS host_metric_samples_sampled_at_idx
ON global.host_metric_samples (sampled_at);
-- +goose StatementEnd
