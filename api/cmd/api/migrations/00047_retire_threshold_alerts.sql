-- +goose Up
-- +goose StatementBegin
-- Issue #1528: the hand-rolled jobs.ThresholdAlertJob is retired. Request /
-- job / frontend p95 latency alerting now lives in Grafana, evaluating real
-- Prometheus histograms (http_request_duration_seconds, job_duration_seconds,
-- web_vitals_seconds) and routing through a provisioned SMTP contact point
-- (infra/grafana/provisioning/alerting/). global.alert_states only ever
-- backed that job, so the whole table goes. The per-class slow-transaction
-- toggles it owned go with it; 'slow_transactions' stays — it still gates
-- IssueNotifierJob and the weekly digest's slow-transaction section.
DROP TABLE IF EXISTS global.alert_states;

DELETE FROM global.notification_settings
WHERE source_key IN (
    'slow_transaction_http_high', 'slow_transaction_job_high',
    'slow_transaction_frontend_high'
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS global.alert_states (
    rule_key TEXT PRIMARY KEY,
    breaching BOOLEAN NOT NULL DEFAULT FALSE,
    since TIMESTAMPTZ,
    last_notified_at TIMESTAMPTZ,
    current_value DOUBLE PRECISION,
    threshold DOUBLE PRECISION
);

INSERT INTO global.notification_settings (source_key) VALUES
('slow_transaction_http_high'),
('slow_transaction_job_high'),
('slow_transaction_frontend_high')
ON CONFLICT (source_key) DO NOTHING;
-- +goose StatementEnd
