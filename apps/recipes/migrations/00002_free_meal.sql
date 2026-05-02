-- +goose Up
-- +goose StatementBegin
ALTER TABLE recipes.plan_meals
DROP CONSTRAINT plan_meals_meal_slot_check,
ALTER COLUMN recipe_id DROP NOT NULL,
ADD COLUMN custom_name TEXT NOT NULL DEFAULT '',
ADD CONSTRAINT plan_meals_meal_slot_check
CHECK (meal_slot IN ('breakfast', 'noon', 'evening')),
ADD CONSTRAINT plan_meals_meal_check
CHECK (recipe_id IS NOT NULL OR custom_name != '');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE recipes.plan_meals
DROP CONSTRAINT plan_meals_meal_check,
DROP CONSTRAINT plan_meals_meal_slot_check,
DROP COLUMN custom_name,
ALTER COLUMN recipe_id SET NOT NULL,
ADD CONSTRAINT plan_meals_meal_slot_check
CHECK (meal_slot IN ('noon', 'evening'));
-- +goose StatementEnd
