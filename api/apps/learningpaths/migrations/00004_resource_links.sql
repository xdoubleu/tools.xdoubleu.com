-- +goose Up
-- +goose StatementBegin
-- No cross-schema FK to books.books or feeds.items: learningpaths never
-- reads another app's schema directly (see docs/adr-0007), so these link
-- IDs are validated at the service layer via Books.GetLibraryBookByID /
-- Feeds.GetItemByID instead of a foreign key.
ALTER TABLE learningpaths.resources
ADD COLUMN IF NOT EXISTS linked_book_id UUID,
ADD COLUMN IF NOT EXISTS linked_feed_item_id UUID;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE learningpaths.resources
DROP COLUMN IF EXISTS linked_book_id,
DROP COLUMN IF EXISTS linked_feed_item_id;
-- +goose StatementEnd
