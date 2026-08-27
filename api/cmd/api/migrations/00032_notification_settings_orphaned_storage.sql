-- +goose Up
-- +goose StatementBegin
-- Orphaned R2 storage objects (books app's daily StorageScanJob) are already
-- surfaced via get_storage_stats but never triggered a notification email —
-- add them as a sixth toggleable source alongside the ones seeded in
-- 00029_notification_settings.sql, 00030_notification_settings_main_ci.sql,
-- and 00031_notification_settings_security_alerts.sql (issue #1274).
INSERT INTO global.notification_settings (source_key) VALUES
('orphaned_storage');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM global.notification_settings
WHERE source_key = 'orphaned_storage';
-- +goose StatementEnd
