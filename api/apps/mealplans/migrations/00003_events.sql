-- +goose Up
-- Events are planning-only meal entries (recipe-less, like custom items) that
-- are excluded from the shopping-list export but still shown on the iCal feed.
ALTER TABLE mealplans.plan_meals
ADD COLUMN is_event BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE mealplans.plan_meals
DROP COLUMN is_event;
