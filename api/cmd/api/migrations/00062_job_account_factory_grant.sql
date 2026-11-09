-- +goose Up
-- +goose StatementBegin
-- The factory_ tools are gated on the factory grant (adr-0047), so a job's
-- account holds it too; the task scope still confines it to its own task.
INSERT INTO global.app_access (user_id, app_name, level)
SELECT
    user_id::TEXT AS user_id,
    'factory' AS app_name,
    'write' AS level
FROM global.agent_accounts
WHERE job_id IS NOT NULL AND revoked_at IS NULL
ON CONFLICT DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM global.app_access
WHERE app_name = 'factory' AND user_id IN (
    SELECT user_id::TEXT
    FROM global.agent_accounts
    WHERE job_id IS NOT NULL
);
-- +goose StatementEnd
