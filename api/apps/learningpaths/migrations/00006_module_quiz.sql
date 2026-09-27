-- +goose Up
-- +goose StatementBegin
-- One quiz per module, carried by the module's terminal checkpoint item.
-- JSONB array of {prompt, options, correct_answer_index}; whole-tree replace
-- so no backfill is needed for the brand-new column.
ALTER TABLE learningpaths.modules
ADD COLUMN IF NOT EXISTS quiz JSONB NOT NULL DEFAULT '[]';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE learningpaths.modules
DROP COLUMN IF EXISTS quiz;
-- +goose StatementEnd