-- The backlog app split into games and books; carry each user's backlog
-- grant over to both new apps.

-- +goose Up
-- +goose StatementBegin
INSERT INTO global.app_access (user_id, app_name)
SELECT
    user_id,
    'games' AS app_name
FROM global.app_access
WHERE app_name = 'backlog'
ON CONFLICT DO NOTHING;

INSERT INTO global.app_access (user_id, app_name)
SELECT
    user_id,
    'books' AS app_name
FROM global.app_access
WHERE app_name = 'backlog'
ON CONFLICT DO NOTHING;

DELETE FROM global.app_access
WHERE app_name = 'backlog';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
INSERT INTO global.app_access (user_id, app_name)
SELECT DISTINCT
    user_id,
    'backlog' AS app_name
FROM global.app_access
WHERE app_name IN ('games', 'books')
ON CONFLICT DO NOTHING;

DELETE FROM global.app_access
WHERE app_name IN ('games', 'books');
-- +goose StatementEnd
