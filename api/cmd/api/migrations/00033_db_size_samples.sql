-- +goose Up
-- +goose StatementBegin
-- Per-table on-disk size time series, sampled daily by
-- internal/observability/jobs.DBSizeSnapshotJob (issue #1282) — the
-- database is reached over a transaction-mode pooler and billed per byte
-- (issue #1027), so "which table is growing fastest" needs a history to
-- answer, not just get_database_stats' live point-in-time snapshot.
CREATE TABLE global.db_size_samples (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    sampled_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    schema_name TEXT NOT NULL,
    table_name TEXT NOT NULL,
    size_bytes BIGINT NOT NULL
);

CREATE INDEX db_size_samples_sampled_at_idx ON global.db_size_samples (
    sampled_at
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS global.db_size_samples;
-- +goose StatementEnd
