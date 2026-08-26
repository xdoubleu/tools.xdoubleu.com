-- +goose Up
-- +goose StatementBegin
-- Security alerts (Dependabot/code-scanning/secret-scanning) are already
-- surfaced on the Issues page (GetSecurityAlerts) but never triggered a
-- notification email — add them as a fifth toggleable source alongside the
-- ones seeded in 00029_notification_settings.sql and
-- 00030_notification_settings_main_ci.sql (issue #1261).
INSERT INTO global.notification_settings (source_key) VALUES
('security_alerts');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM global.notification_settings
WHERE source_key = 'security_alerts';
-- +goose StatementEnd
