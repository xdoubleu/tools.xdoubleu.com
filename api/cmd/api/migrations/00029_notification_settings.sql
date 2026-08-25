-- +goose Up
-- +goose StatementBegin
-- Per-source on/off switch for the admin email notifications sent by
-- jobs.IssueNotifierJob and jobs.WeeklyDigestJob (issue #1214) — until now
-- neither job's notifications could be disabled or were even visible in the
-- monitoring UI.
CREATE TABLE global.notification_settings (
    source_key TEXT PRIMARY KEY,
    enabled BOOLEAN NOT NULL DEFAULT TRUE
);

INSERT INTO global.notification_settings (source_key) VALUES
('sentry_issues'),
('failing_dependency_prs'),
('unhealthy_feeds');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS global.notification_settings;
-- +goose StatementEnd
