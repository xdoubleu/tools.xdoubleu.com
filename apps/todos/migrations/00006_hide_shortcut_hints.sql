-- +goose Up
-- +goose StatementBegin
ALTER TABLE todos.user_settings
ADD COLUMN hide_shortcut_hints BOOLEAN NOT NULL DEFAULT FALSE;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE todos.user_settings
DROP COLUMN hide_shortcut_hints;
-- +goose StatementEnd
