-- +goose Up
ALTER TABLE recipes.plans
ADD COLUMN ical_hide_slots TEXT [] NOT NULL DEFAULT '{}',
ADD COLUMN ical_hide_past BOOLEAN NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE recipes.plans
DROP COLUMN ical_hide_slots,
DROP COLUMN ical_hide_past;
