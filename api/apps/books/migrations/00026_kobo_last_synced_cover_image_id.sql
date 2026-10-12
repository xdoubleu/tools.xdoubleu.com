-- +goose Up
-- +goose StatementBegin
-- The CoverImageId last sent to the Kobo. No backfill: NULL on a book that
-- was synced before means the device may hold an older id, which is what
-- makes the sync handler resend it as a ChangedEntitlement.
ALTER TABLE books.user_books
ADD COLUMN kobo_last_synced_cover_image_id TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE books.user_books
DROP COLUMN kobo_last_synced_cover_image_id;
-- +goose StatementEnd
