-- +goose Up
-- +goose StatementBegin
-- Open (unread, non-dismissed) feed items get their own toggleable source
-- alongside 'unhealthy_feeds' (00029_notification_settings.sql) — the
-- weekly digest email restates unread items still piling up, distinct from
-- feeds that are failing to poll (issue #1355).
INSERT INTO global.notification_settings (source_key) VALUES
('open_feed_items');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM global.notification_settings
WHERE source_key = 'open_feed_items';
-- +goose StatementEnd
