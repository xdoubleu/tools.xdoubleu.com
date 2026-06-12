-- +goose Up
-- +goose StatementBegin
-- Replace per-recipe sharing (recipe_access) with whole-recipe-book sharing.
DROP TABLE IF EXISTS recipes.recipe_access;

CREATE TABLE IF NOT EXISTS recipes.recipebook_access (
    owner_user_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    can_edit BOOL NOT NULL DEFAULT TRUE,
    PRIMARY KEY (owner_user_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_recipebook_access_user
ON recipes.recipebook_access (user_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS recipes.recipebook_access;

CREATE TABLE IF NOT EXISTS recipes.recipe_access (
    recipe_id UUID NOT NULL REFERENCES recipes.recipes (id) ON DELETE CASCADE,
    user_id TEXT NOT NULL,
    PRIMARY KEY (recipe_id, user_id)
);
-- +goose StatementEnd
