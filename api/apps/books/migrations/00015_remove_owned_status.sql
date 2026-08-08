-- +goose Up
-- +goose StatementBegin
-- Issue #866: the "owned" shelf duplicates the intent already covered by
-- "to-read" (want to read). No app code writes "owned" anymore; fold any
-- existing rows back into "to-read".
UPDATE books.user_books SET status = 'to-read'
WHERE status = 'owned';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- No-op: which rows were "owned" vs "to-read" before the Up is not recoverable.
-- +goose StatementEnd
