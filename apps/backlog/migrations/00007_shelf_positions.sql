-- +goose Up
-- +goose StatementBegin
DO $$
DECLARE
    s TEXT;
BEGIN
    SELECT nspname INTO s FROM pg_catalog.pg_namespace
    WHERE nspname IN ('backlog', 'goaltracker')
    ORDER BY (nspname = 'backlog') DESC
    LIMIT 1;

    EXECUTE format(
        'ALTER TABLE %I.user_books ADD COLUMN IF NOT EXISTS shelf_positions JSONB NOT NULL DEFAULT ''{}''',
        s
    );
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
DECLARE
    s TEXT;
BEGIN
    SELECT nspname INTO s FROM pg_catalog.pg_namespace
    WHERE nspname IN ('backlog', 'goaltracker')
    ORDER BY (nspname = 'backlog') DESC
    LIMIT 1;

    EXECUTE format('ALTER TABLE %I.user_books DROP COLUMN IF EXISTS shelf_positions', s);
END;
$$;
-- +goose StatementEnd
