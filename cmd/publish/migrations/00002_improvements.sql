-- +goose Up
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_app_access_user_id ON global.app_access (
    user_id
);

UPDATE global.app_access SET app_name = 'backlog'
WHERE app_name = 'goaltracker';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE global.app_access SET app_name = 'goaltracker'
WHERE app_name = 'backlog';

DROP INDEX IF EXISTS global.idx_app_access_user_id;
-- +goose StatementEnd
