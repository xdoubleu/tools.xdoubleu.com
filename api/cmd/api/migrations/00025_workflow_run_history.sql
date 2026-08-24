-- +goose Up
-- +goose StatementBegin
-- GitHub Actions workflow run history, persisted by
-- internal/observability/jobs.WorkflowRunsSnapshotJob so duration/failure
-- trends survive past internal/github.Client's 45s in-memory cache
-- (issue #1217).
CREATE TABLE global.workflow_run_samples (
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

CREATE INDEX workflow_run_samples_branch_conclusion_idx
ON global.workflow_run_samples (branch, conclusion);

CREATE INDEX workflow_run_samples_started_at_idx
ON global.workflow_run_samples (started_at);

-- Per-job duration breakdown for a recorded run, the "specific actions" data
-- get_workflow_run_stats aggregates for job_duration_stats.
CREATE TABLE global.workflow_job_samples (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    run_id BIGINT NOT NULL,
    job_name TEXT NOT NULL,
    conclusion TEXT NOT NULL,
    duration_ms BIGINT NOT NULL,
    started_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX workflow_job_samples_run_id_idx
ON global.workflow_job_samples (run_id);

CREATE INDEX workflow_job_samples_job_name_idx
ON global.workflow_job_samples (job_name);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS global.workflow_job_samples;
DROP TABLE IF EXISTS global.workflow_run_samples;
-- +goose StatementEnd
