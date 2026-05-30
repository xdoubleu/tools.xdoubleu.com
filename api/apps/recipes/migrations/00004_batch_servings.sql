-- +goose Up
ALTER TABLE recipes.recipes ADD COLUMN batch_servings INT;

-- +goose Down
ALTER TABLE recipes.recipes DROP COLUMN batch_servings;
