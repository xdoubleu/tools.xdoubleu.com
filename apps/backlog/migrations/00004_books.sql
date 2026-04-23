-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS goaltracker.books (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title         VARCHAR(500) NOT NULL,
    authors       VARCHAR(255)[] NOT NULL DEFAULT '{}',
    isbn13        VARCHAR(13),
    isbn10        VARCHAR(10),
    cover_url     VARCHAR(1000),
    description   TEXT,
    external_refs JSONB NOT NULL DEFAULT '{}',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS books_isbn13_idx
    ON goaltracker.books (isbn13)
    WHERE isbn13 IS NOT NULL;

CREATE TABLE IF NOT EXISTS goaltracker.user_books (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     VARCHAR(255) NOT NULL,
    book_id     UUID NOT NULL REFERENCES goaltracker.books(id),
    status      VARCHAR(50) NOT NULL DEFAULT 'wishlist',
    rating      SMALLINT CHECK (rating BETWEEN 1 AND 5),
    notes       TEXT,
    finished_at TIMESTAMPTZ[],
    added_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, book_id)
);

ALTER TABLE goaltracker.user_integrations
    ADD COLUMN IF NOT EXISTS hardcover_api_key TEXT NOT NULL DEFAULT '';

-- Migrate existing goodreads_books → books + user_books
INSERT INTO goaltracker.books (title, authors, external_refs)
SELECT DISTINCT
    g.title,
    ARRAY[g.author],
    jsonb_build_object('goodreads', g.id::text)
FROM goaltracker.goodreads_books g
ON CONFLICT DO NOTHING;

INSERT INTO goaltracker.user_books (user_id, book_id, status, finished_at)
SELECT
    g.user_id,
    b.id,
    CASE g.shelf
        WHEN 'read'              THEN 'finished'
        WHEN 'currently-reading' THEN 'reading'
        ELSE                          'wishlist'
    END,
    g.dates_read
FROM goaltracker.goodreads_books g
JOIN goaltracker.books b
    ON b.title = g.title
    AND b.external_refs->>'goodreads' = g.id::text
ON CONFLICT (user_id, book_id) DO NOTHING;

DROP TABLE IF EXISTS goaltracker.goodreads_books;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS goaltracker.goodreads_books (
    id integer NOT NULL,
    user_id varchar(255) NOT NULL,
    shelf varchar(255) NOT NULL,
    tags varchar(255) [] NOT NULL,
    title varchar(255) NOT NULL,
    author varchar(255) NOT NULL,
    dates_read timestamp [],
    PRIMARY KEY (id, user_id)
);

ALTER TABLE goaltracker.user_integrations
    DROP COLUMN IF EXISTS hardcover_api_key;

DROP TABLE IF EXISTS goaltracker.user_books;
DROP TABLE IF EXISTS goaltracker.books;

-- +goose StatementEnd
