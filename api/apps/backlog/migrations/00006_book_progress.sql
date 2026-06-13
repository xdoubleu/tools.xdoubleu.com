-- +goose Up
-- +goose StatementBegin
ALTER TABLE backlog.books
ADD COLUMN IF NOT EXISTS page_count INTEGER;

ALTER TABLE backlog.user_books
ADD COLUMN IF NOT EXISTS progress_mode TEXT NOT NULL DEFAULT 'pages',
ADD COLUMN IF NOT EXISTS current_page INTEGER NOT NULL DEFAULT 0,
ADD COLUMN IF NOT EXISTS progress_percent SMALLINT NOT NULL DEFAULT 0;

ALTER TABLE backlog.user_books
ADD CONSTRAINT user_books_progress_mode_chk
CHECK (progress_mode IN ('pages', 'percent')),
ADD CONSTRAINT user_books_progress_percent_chk
CHECK (progress_percent BETWEEN 0 AND 100);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE backlog.user_books
DROP CONSTRAINT IF EXISTS user_books_progress_mode_chk,
DROP CONSTRAINT IF EXISTS user_books_progress_percent_chk,
DROP COLUMN IF EXISTS progress_mode,
DROP COLUMN IF EXISTS current_page,
DROP COLUMN IF EXISTS progress_percent;

ALTER TABLE backlog.books
DROP COLUMN IF EXISTS page_count;
-- +goose StatementEnd
