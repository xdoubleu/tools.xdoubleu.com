-- +goose Up
-- +goose StatementBegin
-- Re-runs the one-off backfill so Testing tasks the new path rules cover
-- move to Done.
UPDATE factory.board_settings SET path_rules_backfilled = false;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 1;
-- +goose StatementEnd
