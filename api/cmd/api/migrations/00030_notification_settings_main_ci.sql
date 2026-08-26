-- +goose Up
-- +goose StatementBegin
-- WorkflowRunsSnapshotJob.notifyMainFailure (issue #1217) has always emailed
-- unconditionally, unlike every other admin notification — add it as a
-- fourth toggleable source alongside the ones seeded in
-- 00029_notification_settings.sql.
INSERT INTO global.notification_settings (source_key) VALUES
('failing_main_ci');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM global.notification_settings
WHERE source_key = 'failing_main_ci';
-- +goose StatementEnd
