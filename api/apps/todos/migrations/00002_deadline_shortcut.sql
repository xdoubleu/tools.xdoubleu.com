-- +goose Up
-- +goose StatementBegin
ALTER TABLE todos.tasks ADD COLUMN deadline DATE;
ALTER TABLE todos.url_patterns
ADD COLUMN shortcut TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE todos.tasks DROP COLUMN deadline;
ALTER TABLE todos.url_patterns DROP COLUMN shortcut;
-- +goose StatementEnd
