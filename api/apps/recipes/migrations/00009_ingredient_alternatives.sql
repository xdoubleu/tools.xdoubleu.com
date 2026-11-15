-- +goose Up
-- Ingredients of one recipe sharing an alternative_group are interchangeable
-- options; the first by sort_order is the default. Existing rows stay NULL.
ALTER TABLE recipes.ingredients ADD COLUMN alternative_group TEXT;

-- +goose Down
ALTER TABLE recipes.ingredients DROP COLUMN alternative_group;
