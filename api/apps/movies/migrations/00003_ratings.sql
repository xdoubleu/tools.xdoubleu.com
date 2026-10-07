-- +goose Up
-- +goose StatementBegin
-- 1-5 stars; NULL is unrated, which every existing entry is.
ALTER TABLE movies.user_titles
ADD COLUMN IF NOT EXISTS rating SMALLINT
CHECK (rating IS NULL OR rating BETWEEN 1 AND 5);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE movies.user_titles DROP COLUMN IF EXISTS rating;
-- +goose StatementEnd
