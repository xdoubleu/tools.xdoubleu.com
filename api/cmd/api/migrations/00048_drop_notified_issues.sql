-- +goose Up
-- +goose StatementBegin
-- jobs.IssueNotifierJob is retired (Grafana Phase 3, issue #1541); the
-- realtime issue alerting it did is now Prometheus/Grafana's, so its
-- append-only dedup table has no remaining reader.
DROP TABLE IF EXISTS global.notified_issues;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS global.notified_issues (
    key TEXT PRIMARY KEY,
    notified_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd
