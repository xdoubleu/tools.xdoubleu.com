-- +goose Up
-- +goose StatementBegin
UPDATE global.app_access SET app_name = 'backlog'
WHERE app_name = 'goaltracker';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE global.app_access SET app_name = 'goaltracker'
WHERE app_name = 'backlog';
-- +goose StatementEnd
