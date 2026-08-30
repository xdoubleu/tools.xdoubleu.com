-- +goose Up
-- +goose StatementBegin
-- Per-class slow-transaction threshold rules (issue #1310,
-- jobs.ThresholdAlertJob) get their own toggleable sources, alongside the
-- ones seeded in 00034_notification_settings_alert_rules.sql. Distinct from
-- the flat 'slow_transactions' source seeded in
-- 00036_notification_settings_slow_transactions.sql, which gates
-- IssueNotifierJob/WeeklyDigestJob's email notifications — these three gate
-- ThresholdAlertJob's own breach/recovery emails per transaction class.
INSERT INTO global.notification_settings (source_key) VALUES
('slow_transaction_http_high'),
('slow_transaction_job_high'),
('slow_transaction_frontend_high');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM global.notification_settings
WHERE source_key IN (
    'slow_transaction_http_high', 'slow_transaction_job_high',
    'slow_transaction_frontend_high'
);
-- +goose StatementEnd
