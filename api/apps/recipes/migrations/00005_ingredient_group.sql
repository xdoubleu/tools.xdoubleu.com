-- +goose Up
ALTER TABLE recipes.ingredients ADD COLUMN group_name TEXT;

-- +goose Down
ALTER TABLE recipes.ingredients DROP COLUMN group_name;
