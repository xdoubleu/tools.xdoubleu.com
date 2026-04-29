-- +goose Up
-- +goose StatementBegin
CREATE SCHEMA IF NOT EXISTS todos;

CREATE TABLE IF NOT EXISTS todos.workspaces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id TEXT NOT NULL,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_workspaces_owner ON todos.workspaces (owner_user_id);

CREATE TABLE IF NOT EXISTS todos.sections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id TEXT NOT NULL,
    name TEXT NOT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    workspace_id UUID REFERENCES todos.workspaces (id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_sections_owner ON todos.sections (owner_user_id);

CREATE TABLE IF NOT EXISTS todos.tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    setup_label TEXT NOT NULL DEFAULT '',
    type_label TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'open'
    CHECK (status IN ('open', 'done', 'archived')),
    completed_at TIMESTAMPTZ,
    archived_at TIMESTAMPTZ,
    due_date DATE,
    priority INT NOT NULL DEFAULT 0,
    sort_order INT NOT NULL DEFAULT 0,
    recur_days INT NOT NULL DEFAULT 0,
    section_id UUID REFERENCES todos.sections (id) ON DELETE SET NULL,
    workspace_id UUID REFERENCES todos.workspaces (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_tasks_owner ON todos.tasks (owner_user_id);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON todos.tasks (owner_user_id, status);

CREATE TABLE IF NOT EXISTS todos.task_links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES todos.tasks (id) ON DELETE CASCADE,
    url TEXT NOT NULL,
    label TEXT NOT NULL DEFAULT '',
    sort_order INT NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_task_links_task ON todos.task_links (task_id);

CREATE TABLE IF NOT EXISTS todos.subtasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES todos.tasks (id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    done BOOLEAN NOT NULL DEFAULT FALSE,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_subtasks_task ON todos.subtasks (task_id);

CREATE TABLE IF NOT EXISTS todos.label_presets (
    user_id TEXT NOT NULL,
    category TEXT NOT NULL CHECK (category IN ('setup', 'type')),
    value TEXT NOT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    workspace_id UUID REFERENCES todos.workspaces (id) ON DELETE CASCADE,
    CONSTRAINT label_presets_unique
    UNIQUE NULLS NOT DISTINCT (user_id, category, value, workspace_id)
);

CREATE TABLE IF NOT EXISTS todos.url_patterns (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id TEXT NOT NULL,
    url_prefix TEXT NOT NULL,
    platform_name TEXT NOT NULL,
    type_label TEXT NOT NULL DEFAULT '',
    sort_order INT NOT NULL DEFAULT 0,
    workspace_id UUID REFERENCES todos.workspaces (id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_url_patterns_user ON todos.url_patterns (user_id);

CREATE TABLE IF NOT EXISTS todos.archive_settings (
    user_id TEXT PRIMARY KEY,
    archive_after_hours INT NOT NULL DEFAULT 24
);

CREATE TABLE IF NOT EXISTS todos.policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id TEXT NOT NULL,
    text TEXT NOT NULL,
    reappear_after_hours INT NOT NULL DEFAULT 24,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    workspace_id UUID REFERENCES todos.workspaces (id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_policies_owner ON todos.policies (owner_user_id);

CREATE TABLE IF NOT EXISTS todos.user_settings (
    user_id TEXT PRIMARY KEY,
    active_workspace_id UUID REFERENCES todos.workspaces (id) ON DELETE SET NULL
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP SCHEMA IF EXISTS todos CASCADE;
-- +goose StatementEnd
