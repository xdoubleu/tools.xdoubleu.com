-- +goose Up
-- +goose StatementBegin
-- Grafana Phase 4 (issue #1530): alerting moved wholesale to Grafana
-- (issues #1528, #1541 retired ThresholdAlertJob and IssueNotifierJob), so
-- the email/Slack delivery-channel switch this table backed has no producer
-- left. notifications.Service is now email-only; drop the table and its
-- Slack webhook ciphertext. Prod channel_mode was already 'email'.
DROP TABLE IF EXISTS global.notification_channel_config;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS global.notification_channel_config (
    id BOOLEAN PRIMARY KEY DEFAULT TRUE,
    channel_mode TEXT NOT NULL DEFAULT 'email'
    CHECK (channel_mode IN ('email', 'slack', 'both')),
    slack_webhook_url BYTEA,
    CONSTRAINT notification_channel_config_single_row CHECK (id)
);

INSERT INTO global.notification_channel_config (id) VALUES (TRUE)
ON CONFLICT (id) DO NOTHING;
-- +goose StatementEnd
