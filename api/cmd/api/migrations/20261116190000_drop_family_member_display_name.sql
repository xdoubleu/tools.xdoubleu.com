-- Display names live in global.app_users (user management); the family keeps
-- no copy. Backfill app_users where a member only named themselves in the
-- family, so no name is lost.

-- +goose Up
-- +goose StatementBegin
UPDATE global.app_users u
SET display_name = m.display_name
FROM global.family_members m
WHERE m.user_id = u.id
  AND m.display_name <> ''
  AND COALESCE(u.display_name, '') = '';

ALTER TABLE global.family_members DROP COLUMN IF EXISTS display_name;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE global.family_members
ADD COLUMN IF NOT EXISTS display_name TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd
