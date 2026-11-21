-- +goose Up
-- +goose StatementBegin
-- The person a standing agent account acts for (adr-0057). Existing standing
-- accounts take the admin that created them; job and factory-made ones have
-- none.
ALTER TABLE global.agent_accounts
ADD COLUMN IF NOT EXISTS principal_user_id UUID;

UPDATE global.agent_accounts
SET principal_user_id = substring(created_by FROM 7)::UUID
WHERE
    job_id IS NULL
    AND principal_user_id IS NULL
    AND created_by ~* '^human:[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12}$';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE global.agent_accounts DROP COLUMN IF EXISTS principal_user_id;
-- +goose StatementEnd
