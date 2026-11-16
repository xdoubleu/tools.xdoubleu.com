-- Routines run as factory jobs, which record their own runs, so the Actions
-- run ledger is retired. No backfill: the table only held run history.

-- +goose Up
-- +goose StatementBegin
DROP TABLE IF EXISTS global.automated_actions;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- The dropped run history is not recoverable; the down migration is a no-op.
-- +goose StatementEnd
