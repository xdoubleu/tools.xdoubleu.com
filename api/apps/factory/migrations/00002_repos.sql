-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS factory.repos (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner TEXT NOT NULL,
    name TEXT NOT NULL,
    default_branch TEXT NOT NULL DEFAULT 'main',
    default_harness TEXT NOT NULL DEFAULT 'claude_code' CHECK (
        default_harness IN ('claude_code', 'opencode')
    ),
    setup_commands TEXT NOT NULL DEFAULT '',
    agent_instructions TEXT NOT NULL DEFAULT '',
    time_limit_minutes INT NOT NULL DEFAULT 60 CHECK (
        time_limit_minutes BETWEEN 1 AND 1440
    ),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- GitHub owner and repo names are case-insensitive.
CREATE UNIQUE INDEX IF NOT EXISTS idx_repos_owner_name
ON factory.repos (
    lower(owner), lower(name)
);

INSERT INTO factory.repos (owner, name, agent_instructions)
VALUES ('xdoubleu', 'tools.xdoubleu.com', 'Follow AGENTS.md.')
ON CONFLICT DO NOTHING;

-- Every task so far was filed for this repo, the only one the factory knows,
-- so pointing them all at the seeded row is the whole backfill.
ALTER TABLE factory.tasks
ADD COLUMN IF NOT EXISTS repo_id UUID REFERENCES factory.repos (id);

UPDATE factory.tasks
SET
    repo_id = (
        SELECT id FROM factory.repos
        WHERE owner = 'xdoubleu' AND name = 'tools.xdoubleu.com'
    )
WHERE repo_id IS NULL;

ALTER TABLE factory.tasks ALTER COLUMN repo_id SET NOT NULL;

CREATE INDEX IF NOT EXISTS idx_tasks_repo
ON factory.tasks (
    repo_id
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE factory.tasks DROP COLUMN IF EXISTS repo_id;
DROP TABLE IF EXISTS factory.repos;
-- +goose StatementEnd
