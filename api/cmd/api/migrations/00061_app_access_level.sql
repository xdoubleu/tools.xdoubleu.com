-- +goose Up
-- +goose StatementBegin
-- Agent app access (adr-0047): a grant is read or write. Existing rows are
-- people's, whose access has always allowed both, so they default to write and
-- need no backfill. Standing agent accounts keep the factory tools they have.
ALTER TABLE global.app_access
ADD COLUMN IF NOT EXISTS level TEXT NOT NULL DEFAULT 'write',
ADD CONSTRAINT app_access_level_check CHECK (level IN ('read', 'write'));

INSERT INTO global.app_access (user_id, app_name)
SELECT
    user_id::TEXT AS user_id,
    'factory' AS app_name
FROM global.agent_accounts
WHERE job_id IS NULL AND revoked_at IS NULL
ON CONFLICT DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM global.app_access
WHERE user_id IN (SELECT user_id::TEXT FROM global.agent_accounts);
ALTER TABLE global.app_access
DROP CONSTRAINT IF EXISTS app_access_level_check,
DROP COLUMN IF EXISTS level;
-- +goose StatementEnd
