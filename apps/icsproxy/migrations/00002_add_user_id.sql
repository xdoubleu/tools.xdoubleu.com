-- +goose Up
-- +goose StatementBegin
ALTER TABLE icsproxy.feeds
ADD COLUMN user_id TEXT NOT NULL DEFAULT '';

DO $$
DECLARE
    first_user_id TEXT;
BEGIN
    SELECT id INTO first_user_id
    FROM global.app_users
    ORDER BY last_seen DESC
    LIMIT 1;

    IF first_user_id IS NOT NULL THEN
        UPDATE icsproxy.feeds
        SET user_id = first_user_id
        WHERE user_id = '';
    END IF;
END $$;

ALTER TABLE icsproxy.feeds ALTER COLUMN user_id DROP DEFAULT;

CREATE INDEX IF NOT EXISTS feeds_user_id_idx ON icsproxy.feeds (user_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS icsproxy.feeds_user_id_idx;
ALTER TABLE icsproxy.feeds DROP COLUMN user_id;
-- +goose StatementEnd
