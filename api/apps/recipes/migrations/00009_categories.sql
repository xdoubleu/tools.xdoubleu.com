-- +goose Up
-- Empty means uncategorised; existing recipes keep that, so no backfill.
ALTER TABLE recipes.recipes
    ADD COLUMN meal_type TEXT NOT NULL DEFAULT ''
        CHECK (meal_type IN ('', 'light_meal', 'main_meal')),
    ADD COLUMN effort TEXT NOT NULL DEFAULT ''
        CHECK (effort IN ('', 'low', 'high'));

-- +goose Down
ALTER TABLE recipes.recipes DROP COLUMN effort, DROP COLUMN meal_type;
