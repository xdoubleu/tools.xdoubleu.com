-- +goose Up
-- +goose StatementBegin
-- Book-linked items derive completion from the linked book at 100% read, so
-- the link must live on the item itself (resources already carry
-- linked_book_id for read-only display). Validated at the service layer like
-- resource links; no cross-schema FK.
ALTER TABLE learningpaths.items
ADD COLUMN IF NOT EXISTS linked_book_id UUID;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE learningpaths.items
DROP COLUMN IF EXISTS linked_book_id;
-- +goose StatementEnd
