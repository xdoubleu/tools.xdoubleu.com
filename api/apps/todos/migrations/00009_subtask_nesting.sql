-- +goose Up
-- +goose StatementBegin
ALTER TABLE todos.subtasks
ADD COLUMN parent_subtask_id UUID REFERENCES todos.subtasks (
    id
) ON DELETE CASCADE,
ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

CREATE INDEX IF NOT EXISTS idx_subtasks_parent ON todos.subtasks (
    parent_subtask_id
);

CREATE OR REPLACE FUNCTION todos.set_updated_at()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN NEW.updated_at = now(); RETURN NEW; END;
$$;

DROP TRIGGER IF EXISTS trg_subtasks_updated_at ON todos.subtasks;
CREATE TRIGGER trg_subtasks_updated_at
BEFORE UPDATE ON todos.subtasks
FOR EACH ROW EXECUTE FUNCTION todos.set_updated_at();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_subtasks_updated_at ON todos.subtasks;
DROP FUNCTION IF EXISTS todos.set_updated_at;
DROP INDEX IF EXISTS idx_subtasks_parent;
ALTER TABLE todos.subtasks
DROP COLUMN IF EXISTS updated_at,
DROP COLUMN IF EXISTS parent_subtask_id;
-- +goose StatementEnd
