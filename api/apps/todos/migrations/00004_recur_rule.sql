-- +goose Up
-- +goose StatementBegin
ALTER TABLE todos.tasks
ADD COLUMN recur_rule TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE todos.tasks
DROP COLUMN recur_rule;
-- +goose StatementEnd
