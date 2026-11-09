-- +goose Up
-- Meals sharing a date and slot are listed in creation order. Existing rows
-- all get the migration time and fall back to ordering by id, which is
-- arbitrary but stable.
ALTER TABLE mealplans.plan_meals
ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- +goose Down
ALTER TABLE mealplans.plan_meals
DROP COLUMN created_at;
