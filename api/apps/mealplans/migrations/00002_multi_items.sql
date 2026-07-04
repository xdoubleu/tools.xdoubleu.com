-- +goose Up
-- Allow multiple meals per slot (e.g. custom items alongside a recipe).
ALTER TABLE mealplans.plan_meals
DROP CONSTRAINT plan_meals_plan_id_meal_date_meal_slot_key;

-- +goose Down
ALTER TABLE mealplans.plan_meals
ADD CONSTRAINT plan_meals_plan_id_meal_date_meal_slot_key
UNIQUE (plan_id, meal_date, meal_slot);
