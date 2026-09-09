-- +goose Up
-- +goose StatementBegin
-- Global delivery-channel switch for the monitoring alerts sent by
-- jobs.IssueNotifierJob and jobs.ThresholdAlertJob (issue #1482) — until now
-- every alert went out over email only. channel_mode picks email, Slack, or
-- both; slack_webhook_url holds a crypto.Sealer ciphertext of the Slack
-- Incoming Webhook URL (NULL = unset), set entirely through the monitoring
-- settings UI rather than a deploy secret. Single row, guarded by the id
-- CHECK. Weekly digests stay email-only regardless of this setting
-- (docs/adr-0020).
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

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS global.notification_channel_config;
-- +goose StatementEnd
