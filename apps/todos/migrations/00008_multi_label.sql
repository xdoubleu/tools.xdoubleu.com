-- +goose Up
-- +goose StatementBegin
ALTER TABLE todos.label_presets ADD COLUMN color TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE todos.tasks ADD COLUMN labels TEXT [] NOT NULL DEFAULT '{}';
UPDATE todos.tasks SET
    labels = CASE WHEN label = '' THEN '{}' ELSE ARRAY[label] END;
ALTER TABLE todos.tasks DROP COLUMN label;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE todos.subtasks ADD COLUMN labels TEXT [] NOT NULL DEFAULT '{}';
UPDATE todos.subtasks SET
    labels = CASE WHEN label = '' THEN '{}' ELSE ARRAY[label] END;
ALTER TABLE todos.subtasks DROP COLUMN label;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE todos.subtasks ADD COLUMN label TEXT NOT NULL DEFAULT '';
UPDATE todos.subtasks SET label = COALESCE(labels[1], '');
ALTER TABLE todos.subtasks DROP COLUMN labels;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE todos.tasks ADD COLUMN label TEXT NOT NULL DEFAULT '';
UPDATE todos.tasks SET label = COALESCE(labels[1], '');
ALTER TABLE todos.tasks DROP COLUMN labels;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE todos.label_presets DROP COLUMN color;
-- +goose StatementEnd
