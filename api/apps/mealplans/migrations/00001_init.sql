-- +goose Up
-- +goose StatementBegin
CREATE SCHEMA IF NOT EXISTS mealplans;

-- Move plan tables from recipes schema (created by recipes app migrations).
ALTER TABLE recipes.plans SET SCHEMA mealplans;
ALTER TABLE recipes.plan_access SET SCHEMA mealplans;
ALTER TABLE recipes.plan_meals SET SCHEMA mealplans;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE mealplans.plan_meals SET SCHEMA recipes;
ALTER TABLE mealplans.plan_access SET SCHEMA recipes;
ALTER TABLE mealplans.plans SET SCHEMA recipes;
DROP SCHEMA IF EXISTS mealplans;
-- +goose StatementEnd
