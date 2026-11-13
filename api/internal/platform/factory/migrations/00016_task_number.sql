-- +goose Up
-- +goose StatementBegin
-- The human-readable task id (T-142). The UUID stays the key; the number is
-- what people, branches and pull requests use. Existing rows are numbered
-- oldest first.
CREATE SEQUENCE IF NOT EXISTS factory.task_number_seq;

ALTER TABLE factory.tasks ADD COLUMN IF NOT EXISTS number BIGINT;

UPDATE factory.tasks t SET number = n.rn
FROM (
    SELECT
        id,
        row_number() OVER (ORDER BY created_at, id) AS rn
    FROM factory.tasks
) AS n
WHERE t.id = n.id AND t.number IS NULL;

SELECT setval(
    'factory.task_number_seq',
    coalesce((SELECT max(number) FROM factory.tasks), 0) + 1,
    FALSE
);

ALTER TABLE factory.tasks
ALTER COLUMN number SET DEFAULT nextval('factory.task_number_seq'),
ALTER COLUMN number SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_tasks_number ON factory.tasks (number);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS factory.idx_tasks_number;
ALTER TABLE factory.tasks DROP COLUMN IF EXISTS number;
DROP SEQUENCE IF EXISTS factory.task_number_seq;
-- +goose StatementEnd
