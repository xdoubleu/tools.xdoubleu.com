-- +goose Up
-- +goose StatementBegin
ALTER TABLE todos.tasks ADD COLUMN label TEXT NOT NULL DEFAULT '';
UPDATE todos.tasks SET label = COALESCE(NULLIF(type_label, ''), NULLIF(setup_label, ''), '');
ALTER TABLE todos.tasks DROP COLUMN setup_label;
ALTER TABLE todos.tasks DROP COLUMN type_label;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE todos.label_presets DROP CONSTRAINT label_presets_unique;
ALTER TABLE todos.label_presets DROP CONSTRAINT label_presets_category_check;
DELETE FROM todos.label_presets lp1
USING todos.label_presets lp2
WHERE lp1.ctid > lp2.ctid
  AND lp1.user_id = lp2.user_id
  AND lp1.value = lp2.value
  AND lp1.workspace_id IS NOT DISTINCT FROM lp2.workspace_id;
UPDATE todos.label_presets SET category = 'label';
ALTER TABLE todos.label_presets
  ADD CONSTRAINT label_presets_category_check CHECK (category IN ('label'));
ALTER TABLE todos.label_presets
  ADD CONSTRAINT label_presets_unique
  UNIQUE NULLS NOT DISTINCT (user_id, category, value, workspace_id);
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE todos.url_patterns RENAME COLUMN type_label TO label;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE todos.url_patterns RENAME COLUMN label TO type_label;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE todos.label_presets DROP CONSTRAINT label_presets_unique;
ALTER TABLE todos.label_presets DROP CONSTRAINT label_presets_category_check;
UPDATE todos.label_presets SET category = 'type';
ALTER TABLE todos.label_presets
  ADD CONSTRAINT label_presets_category_check CHECK (category IN ('setup', 'type'));
ALTER TABLE todos.label_presets
  ADD CONSTRAINT label_presets_unique
  UNIQUE NULLS NOT DISTINCT (user_id, category, value, workspace_id);
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE todos.tasks ADD COLUMN setup_label TEXT NOT NULL DEFAULT '';
ALTER TABLE todos.tasks ADD COLUMN type_label TEXT NOT NULL DEFAULT '';
UPDATE todos.tasks SET type_label = label;
ALTER TABLE todos.tasks DROP COLUMN label;
-- +goose StatementEnd
