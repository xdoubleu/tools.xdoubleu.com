-- +goose Up
ALTER TABLE recipes.recipes ADD COLUMN is_draft BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE recipes.recipes DROP COLUMN is_draft;
