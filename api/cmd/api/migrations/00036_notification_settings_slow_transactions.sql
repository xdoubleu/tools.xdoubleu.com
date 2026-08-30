-- +goose Up
-- +goose StatementBegin
-- Slow transactions were already surfaced on /monitoring/observability
-- (GetSlowTransactions) but never notified anywhere — add them as a
-- toggleable source alongside the ones in 00029_notification_settings.sql,
-- 00030_notification_settings_main_ci.sql, and
-- 00031_notification_settings_security_alerts.sql (issue #1308).
INSERT INTO global.notification_settings (source_key) VALUES
('slow_transactions');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM global.notification_settings
WHERE source_key = 'slow_transactions';
-- +goose StatementEnd
