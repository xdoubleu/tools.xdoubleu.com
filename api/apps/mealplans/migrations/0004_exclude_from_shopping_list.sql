-- +goose Up
-- The former "event" flag is now a per-entry export toggle: a custom
-- (recipe-less) meal can be kept off the shopping list while still showing on
-- the calendar and iCal feed.
ALTER TABLE mealplans.plan_meals
RENAME COLUMN is_event TO exclude_from_shopping_list;

-- +goose Down
ALTER TABLE mealplans.plan_meals
RENAME COLUMN exclude_from_shopping_list TO is_event;
