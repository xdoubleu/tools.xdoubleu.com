-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS "uuid-ossp"; -- noqa: L057

CREATE SCHEMA goaltracker;

CREATE TABLE IF NOT EXISTS goaltracker.states (
    id varchar(255) NOT NULL,
    user_id varchar(255) NOT NULL,
    name varchar(255) NOT NULL,
    "order" integer NOT NULL,
    PRIMARY KEY (id, user_id)
);

CREATE TABLE IF NOT EXISTS goaltracker.goals (
    id varchar(255) NOT NULL,
    user_id varchar(255) NOT NULL,
    parent_id varchar(255),
    name varchar(255) NOT NULL,
    state_id varchar(255) NOT NULL,
    "order" integer NOT NULL,
    is_linked boolean NOT NULL DEFAULT false,
    source_id integer,
    type_id integer,
    target_value integer,
    period integer,
    due_time timestamp,
    config json,
    PRIMARY KEY (id, user_id)
);

CREATE TABLE IF NOT EXISTS goaltracker.progress (
    type_id integer NOT NULL,
    user_id varchar(255) NOT NULL,
    date timestamp NOT NULL,
    value varchar(255) NOT NULL,
    PRIMARY KEY (type_id, user_id, date)
);

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

CREATE TABLE IF NOT EXISTS goaltracker.steam_games (
    id integer NOT NULL,
    user_id varchar(255) NOT NULL,
    name varchar(255) NOT NULL,
    is_delisted boolean NOT NULL DEFAULT false,
    completion_rate varchar(255) DEFAULT '',
    contribution varchar(255) DEFAULT '',
    PRIMARY KEY (id, user_id)
);

CREATE TABLE IF NOT EXISTS goaltracker.steam_achievements (
    name varchar(255) NOT NULL,
    user_id varchar(255) NOT NULL,
    game_id integer NOT NULL,
    achieved boolean NOT NULL DEFAULT false,
    unlock_time timestamp,
    PRIMARY KEY (name, user_id, game_id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP SCHEMA IF EXISTS goaltracker CASCADE;
-- +goose StatementEnd
