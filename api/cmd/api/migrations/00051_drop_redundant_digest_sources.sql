-- +goose Up
-- +goose StatementBegin
-- Issue #1597: sentry_issues, failing_dependency_prs, security_alerts, and
-- slow_transactions are dropped from WeeklyDigestJob's send() — Grafana now
-- alerts on all four in real time and delivers to Slack (adr-0022), making
-- the weekly restatement redundant. unhealthy_feeds and open_feed_items
-- stay: they're personal reading/feed-backlog reminders with no Grafana
-- equivalent (Grafana only watches infra metrics).
DELETE FROM global.notification_settings
WHERE source_key IN (
    'sentry_issues', 'failing_dependency_prs', 'security_alerts',
    'slow_transactions'
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
INSERT INTO global.notification_settings (source_key) VALUES
('sentry_issues'),
('failing_dependency_prs'),
('security_alerts'),
('slow_transactions')
ON CONFLICT (source_key) DO NOTHING;
-- +goose StatementEnd
