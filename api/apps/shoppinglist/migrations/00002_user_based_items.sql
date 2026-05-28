-- +goose Up
-- +goose StatementBegin
TRUNCATE shoppinglist.custom_items;

ALTER TABLE shoppinglist.custom_items
DROP COLUMN plan_id CASCADE,
ADD COLUMN user_id TEXT NOT NULL;

DROP INDEX IF EXISTS shoppinglist.idx_custom_items_plan;

CREATE INDEX idx_custom_items_user ON shoppinglist.custom_items (user_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
TRUNCATE shoppinglist.custom_items;

DROP INDEX IF EXISTS shoppinglist.idx_custom_items_user;

ALTER TABLE shoppinglist.custom_items
DROP COLUMN user_id,
ADD COLUMN plan_id UUID NOT NULL REFERENCES mealplans.plans (
    id
) ON DELETE CASCADE;

CREATE INDEX idx_custom_items_plan ON shoppinglist.custom_items (plan_id);
-- +goose StatementEnd
