-- +goose Up
-- +goose StatementBegin

ALTER TABLE goaltracker.user_books
ADD COLUMN IF NOT EXISTS tags VARCHAR(255) [] NOT NULL DEFAULT '{}';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE goaltracker.user_books
DROP COLUMN IF EXISTS tags;

-- +goose StatementEnd
