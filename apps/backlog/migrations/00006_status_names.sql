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

    EXECUTE format('UPDATE %I.user_books SET status = ''to-read''           WHERE status = ''wishlist''',   s);
    EXECUTE format('UPDATE %I.user_books SET status = ''currently-reading'' WHERE status = ''reading''',    s);
    EXECUTE format('UPDATE %I.user_books SET status = ''read''              WHERE status = ''finished''',   s);
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

    EXECUTE format('UPDATE %I.user_books SET status = ''wishlist''  WHERE status = ''to-read''',           s);
    EXECUTE format('UPDATE %I.user_books SET status = ''reading''   WHERE status = ''currently-reading''', s);
    EXECUTE format('UPDATE %I.user_books SET status = ''finished''  WHERE status = ''read''',              s);
END;
$$;
-- +goose StatementEnd
