-- Tracks the RevisionId last sent to the Kobo device for a synced book, so
-- the sync handler can tell "never synced" (send NewEntitlement) apart from
-- "synced with a different revision" (send ChangedEntitlement, replacing the
-- existing entitlement instead of adding a duplicate when a KEPUB is
-- regenerated) apart from "synced, unchanged" (issue #1734).

-- +goose Up
-- +goose StatementBegin
ALTER TABLE books.user_books
ADD COLUMN kobo_last_synced_revision TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE books.user_books DROP COLUMN kobo_last_synced_revision;
-- +goose StatementEnd
