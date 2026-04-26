-- +goose Up
-- +goose StatementBegin
CREATE SCHEMA IF NOT EXISTS backlog;

CREATE TABLE IF NOT EXISTS backlog.books (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title TEXT NOT NULL,
    authors TEXT [] NOT NULL DEFAULT '{}',
    isbn13 TEXT,
    isbn10 TEXT,
    cover_url TEXT,
    description TEXT,
    external_refs JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS books_isbn13_idx ON backlog.books (
    isbn13
) WHERE isbn13 IS NOT NULL;

CREATE TABLE IF NOT EXISTS backlog.user_books (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id TEXT NOT NULL REFERENCES global.app_users (id) ON DELETE CASCADE,
    book_id UUID NOT NULL REFERENCES backlog.books (id) ON DELETE CASCADE,
    status TEXT NOT NULL,
    rating SMALLINT CHECK (rating BETWEEN 1 AND 5),
    notes TEXT,
    finished_at TIMESTAMPTZ [],
    tags TEXT [] DEFAULT '{}',
    shelf_positions JSONB NOT NULL DEFAULT '{}',
    added_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, book_id)
);

CREATE INDEX IF NOT EXISTS idx_user_books_user_id ON backlog.user_books (
    user_id
);
CREATE INDEX IF NOT EXISTS idx_user_books_status ON backlog.user_books (status);

CREATE TABLE IF NOT EXISTS backlog.steam_games (
    id BIGINT NOT NULL,
    user_id TEXT NOT NULL REFERENCES global.app_users (id) ON DELETE CASCADE,
    name TEXT NOT NULL, is_delisted BOOL NOT NULL DEFAULT FALSE,
    completion_rate VARCHAR NOT NULL DEFAULT '0.00',
    contribution REAL NOT NULL DEFAULT 0,
    playtime_forever BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_steam_games_user_id ON backlog.steam_games (
    user_id
);

CREATE TABLE IF NOT EXISTS backlog.steam_achievements (
    name TEXT NOT NULL,
    user_id TEXT NOT NULL REFERENCES global.app_users (id) ON DELETE CASCADE,
    game_id BIGINT NOT NULL,
    achieved BOOL NOT NULL DEFAULT FALSE,
    unlock_time TIMESTAMPTZ,
    PRIMARY KEY (name, user_id, game_id),
    FOREIGN KEY (game_id, user_id) REFERENCES backlog.steam_games (
        id, user_id
    ) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS backlog.progress (
    type_id TEXT NOT NULL,
    user_id TEXT NOT NULL REFERENCES global.app_users (id) ON DELETE CASCADE,
    date DATE NOT NULL,
    value VARCHAR NOT NULL,
    PRIMARY KEY (type_id, user_id, date)
);

CREATE INDEX IF NOT EXISTS idx_progress_user_date ON backlog.progress (
    user_id, date
);

CREATE TABLE IF NOT EXISTS backlog.user_integrations (
    user_id TEXT PRIMARY KEY REFERENCES global.app_users (id) ON DELETE CASCADE,
    steam_api_key TEXT,
    steam_user_id TEXT,
    goodreads_url TEXT,
    hardcover_api_key TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP SCHEMA IF EXISTS backlog CASCADE;
-- +goose StatementEnd
