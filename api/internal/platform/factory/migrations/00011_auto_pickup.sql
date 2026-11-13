-- +goose Up
-- +goose StatementBegin
-- A repo opts in to the runner taking its Ready tasks without a press of Run.
-- Off by default, so no backfill.
ALTER TABLE factory.repos
ADD COLUMN IF NOT EXISTS auto_pickup BOOLEAN NOT NULL DEFAULT false;

-- What a human approved when moving a task to Ready: its title and body then.
-- Auto pickup runs this, not the current text, so an agent that edits the task
-- later can't change what runs. Null when the task is not Ready or its body was
-- last edited outside a browser. Tasks already Ready stay null: they were never
-- snapshotted, so a human has to move them to Ready again.
ALTER TABLE factory.tasks
ADD COLUMN IF NOT EXISTS ready_title TEXT,
ADD COLUMN IF NOT EXISTS ready_body TEXT;

-- Board-wide switches; one row.
CREATE TABLE IF NOT EXISTS factory.board_settings (
    id BOOLEAN PRIMARY KEY DEFAULT true CHECK (id),
    pickup_paused BOOLEAN NOT NULL DEFAULT false
);
INSERT INTO factory.board_settings (id) VALUES (true) ON CONFLICT DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS factory.board_settings;
ALTER TABLE factory.tasks
DROP COLUMN IF EXISTS ready_body,
DROP COLUMN IF EXISTS ready_title;
ALTER TABLE factory.repos DROP COLUMN IF EXISTS auto_pickup;
-- +goose StatementEnd
