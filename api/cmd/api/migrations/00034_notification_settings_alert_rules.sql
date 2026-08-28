-- +goose Up
-- +goose StatementBegin
-- Threshold alert rules (issue #1283, jobs.ThresholdAlertJob) get their own
-- toggleable sources alongside the ones seeded in
-- 00029_notification_settings.sql, 00030_notification_settings_main_ci.sql,
-- 00031_notification_settings_security_alerts.sql, and
-- 00032_notification_settings_orphaned_storage.sql. db_size_high isn't
-- seeded here — it depends on issue #1282's db_size_samples table, which
-- doesn't exist yet.
INSERT INTO global.notification_settings (source_key) VALUES
('host_cpu_high'),
('host_memory_high'),
('host_disk_high'),
('r2_usage_high'),
('ci_duration_high');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM global.notification_settings
WHERE source_key IN (
    'host_cpu_high', 'host_memory_high', 'host_disk_high',
    'r2_usage_high', 'ci_duration_high'
);
-- +goose StatementEnd
