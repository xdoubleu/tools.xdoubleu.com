-- Adopt the Steam tables from the former backlog schema when it exists
-- (production upgrade path), or create them fresh (new installations).
-- The games schema itself is created by ApplyMigrationsFromFS before goose
-- runs. Shapes match the final backlog state (migrations 00001-00019).

-- +goose Up
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_schema = 'backlog' AND table_name = 'steam_games'
    ) THEN
        ALTER TABLE backlog.steam_games SET SCHEMA games;
        ALTER TABLE backlog.steam_achievements SET SCHEMA games;
    ELSE
        CREATE TABLE games.steam_games (
            id BIGINT NOT NULL,
            user_id TEXT NOT NULL,
            name TEXT NOT NULL,
            is_delisted BOOL NOT NULL DEFAULT FALSE,
            completion_rate VARCHAR NOT NULL DEFAULT '0.00',
            contribution REAL NOT NULL DEFAULT 0,
            playtime_forever BIGINT NOT NULL DEFAULT 0,
            image_url TEXT NOT NULL DEFAULT '',
            last_synced_at TIMESTAMPTZ NOT NULL DEFAULT now(),
            PRIMARY KEY (id, user_id)
        );

        CREATE INDEX idx_steam_games_user_id ON games.steam_games (user_id);

        CREATE TABLE games.steam_achievements (
            name TEXT NOT NULL,
            user_id TEXT NOT NULL,
            game_id BIGINT NOT NULL,
            achieved BOOL NOT NULL DEFAULT FALSE,
            unlock_time TIMESTAMPTZ,
            display_name TEXT NOT NULL DEFAULT '',
            description TEXT NOT NULL DEFAULT '',
            global_percent NUMERIC(6, 2),
            icon_url TEXT NOT NULL DEFAULT '',
            PRIMARY KEY (name, user_id, game_id),
            FOREIGN KEY (game_id, user_id) REFERENCES games.steam_games (
                id, user_id
            ) ON DELETE CASCADE
        );
    END IF;
END $$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS games.progress (
    type_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    date DATE NOT NULL,
    value VARCHAR NOT NULL,
    PRIMARY KEY (type_id, user_id, date)
);

CREATE INDEX IF NOT EXISTS idx_progress_user_date ON games.progress (
    user_id, date
);

CREATE TABLE IF NOT EXISTS games.user_integrations (
    user_id TEXT PRIMARY KEY,
    steam_user_id TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE OR REPLACE FUNCTION games.set_updated_at()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN NEW.updated_at = now(); RETURN NEW; END;
$$;

DROP TRIGGER IF EXISTS trg_user_integrations_updated_at
ON games.user_integrations;
CREATE TRIGGER trg_user_integrations_updated_at
BEFORE UPDATE ON games.user_integrations
FOR EACH ROW EXECUTE FUNCTION games.set_updated_at();
-- +goose StatementEnd

-- Copy the Steam rows out of the shared backlog tables. Steam progress rows
-- are keyed by type_id '0'; user_integrations is copied (not moved) because
-- the backlog row also carried book-side columns that die with the schema.
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_schema = 'backlog' AND table_name = 'progress'
    ) THEN
        INSERT INTO games.progress (type_id, user_id, date, value)
        SELECT type_id, user_id, date, value
        FROM backlog.progress
        WHERE type_id = '0'
        ON CONFLICT DO NOTHING;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_schema = 'backlog' AND table_name = 'user_integrations'
    ) THEN
        INSERT INTO games.user_integrations (user_id, steam_user_id, updated_at)
        SELECT user_id, steam_user_id, updated_at
        FROM backlog.user_integrations
        ON CONFLICT DO NOTHING;
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- Not reversible: the backlog schema no longer exists to move tables back to.
-- Restore from a database backup instead.
-- +goose StatementBegin
SELECT 1;
-- +goose StatementEnd
