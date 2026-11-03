-- +goose Up
-- +goose StatementBegin
-- A job's own agent account (adr-0039) is confined to the task the job runs
-- on and lives as long as the job. Existing accounts are unscoped, so no
-- backfill. The ids are not foreign keys: jobs belong to the factory schema.
ALTER TABLE global.agent_accounts
ADD COLUMN IF NOT EXISTS job_id UUID UNIQUE,
ADD COLUMN IF NOT EXISTS task_id UUID,
ADD CONSTRAINT agent_accounts_scope_check
CHECK ((job_id IS NULL) = (task_id IS NULL));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM auth.oauth2_clients
WHERE id IN (
    SELECT client_id FROM global.agent_accounts
    WHERE job_id IS NOT NULL
);
DELETE FROM global.app_users
WHERE id IN (
    SELECT user_id FROM global.agent_accounts
    WHERE job_id IS NOT NULL
);
DELETE FROM auth.users
WHERE id IN (
    SELECT user_id FROM global.agent_accounts
    WHERE job_id IS NOT NULL
);
DELETE FROM global.agent_accounts
WHERE job_id IS NOT NULL;
ALTER TABLE global.agent_accounts
DROP CONSTRAINT IF EXISTS agent_accounts_scope_check,
DROP COLUMN IF EXISTS task_id,
DROP COLUMN IF EXISTS job_id;
-- +goose StatementEnd
