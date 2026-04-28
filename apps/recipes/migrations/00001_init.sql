-- +goose Up
-- +goose StatementBegin
CREATE SCHEMA IF NOT EXISTS recipes;

CREATE TABLE IF NOT EXISTS recipes.recipes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id TEXT NOT NULL,
    name TEXT NOT NULL,
    instructions TEXT NOT NULL DEFAULT '',
    base_servings INT NOT NULL DEFAULT 2,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_recipes_user_id ON recipes.recipes (user_id);

CREATE TABLE IF NOT EXISTS recipes.recipe_access (
    recipe_id UUID NOT NULL REFERENCES recipes.recipes (id) ON DELETE CASCADE,
    user_id TEXT NOT NULL,
    PRIMARY KEY (recipe_id, user_id)
);

CREATE TABLE IF NOT EXISTS recipes.ingredients (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    recipe_id UUID NOT NULL REFERENCES recipes.recipes (id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    amount NUMERIC(10, 4) NOT NULL,
    unit TEXT NOT NULL DEFAULT '',
    sort_order INT NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_ingredients_recipe_id ON recipes.ingredients (
    recipe_id
);

CREATE TABLE IF NOT EXISTS recipes.plans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id TEXT NOT NULL,
    name TEXT NOT NULL,
    ical_token UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_plans_owner ON recipes.plans (owner_user_id);

CREATE TABLE IF NOT EXISTS recipes.plan_access (
    plan_id UUID NOT NULL REFERENCES recipes.plans (id) ON DELETE CASCADE,
    user_id TEXT NOT NULL,
    can_edit BOOL NOT NULL DEFAULT FALSE,
    PRIMARY KEY (plan_id, user_id)
);

CREATE TABLE IF NOT EXISTS recipes.plan_meals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plan_id UUID NOT NULL REFERENCES recipes.plans (id) ON DELETE CASCADE,
    meal_date DATE NOT NULL,
    meal_slot TEXT NOT NULL CHECK (meal_slot IN ('noon', 'evening')),
    recipe_id UUID NOT NULL REFERENCES recipes.recipes (id),
    servings INT NOT NULL DEFAULT 2,
    UNIQUE (plan_id, meal_date, meal_slot)
);
CREATE INDEX IF NOT EXISTS idx_plan_meals_plan_id ON recipes.plan_meals (
    plan_id
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP SCHEMA IF EXISTS recipes CASCADE;
-- +goose StatementEnd
