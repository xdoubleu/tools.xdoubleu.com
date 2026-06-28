-- +goose Up
ALTER TABLE backlog.books DROP COLUMN isbn10;
ALTER TABLE backlog.books DROP COLUMN external_refs;

-- +goose Down
ALTER TABLE backlog.books ADD COLUMN isbn10 TEXT;
ALTER TABLE backlog.books ADD COLUMN external_refs JSONB NOT NULL DEFAULT '{}';
