-- +goose Up
-- +goose StatementBegin
CREATE SCHEMA IF NOT EXISTS shoppinglist;

CREATE TABLE IF NOT EXISTS shoppinglist.custom_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plan_id UUID NOT NULL REFERENCES mealplans.plans (id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    amount NUMERIC NOT NULL DEFAULT 0,
    unit TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_custom_items_plan ON shoppinglist.custom_items (
    plan_id
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS shoppinglist.custom_items;
DROP SCHEMA IF EXISTS shoppinglist;
-- +goose StatementEnd
