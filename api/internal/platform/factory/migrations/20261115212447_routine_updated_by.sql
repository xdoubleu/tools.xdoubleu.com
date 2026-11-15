-- +goose Up
-- +goose StatementBegin
-- Who last saved a routine, as "<kind>:<id>". Existing rows were saved by a
-- person, whom no one recorded, so they stay empty.
ALTER TABLE factory.routines
ADD COLUMN IF NOT EXISTS updated_by TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE factory.routines DROP COLUMN IF EXISTS updated_by;
-- +goose StatementEnd
