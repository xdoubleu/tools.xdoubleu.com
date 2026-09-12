-- +goose Up
-- +goose StatementBegin
CREATE SCHEMA IF NOT EXISTS learningpaths;

CREATE TABLE IF NOT EXISTS learningpaths.learning_paths (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id TEXT NOT NULL,
    title TEXT NOT NULL,
    goal TEXT NOT NULL DEFAULT '',
    routine TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_learning_paths_user_id
ON learningpaths.learning_paths (
    user_id
);

CREATE TABLE IF NOT EXISTS learningpaths.modules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    learning_path_id UUID NOT NULL REFERENCES learningpaths.learning_paths (
        id
    ) ON DELETE CASCADE,
    title TEXT NOT NULL,
    sort_order INT NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_modules_learning_path_id
ON learningpaths.modules (
    learning_path_id
);

CREATE TABLE IF NOT EXISTS learningpaths.items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    module_id UUID NOT NULL REFERENCES learningpaths.modules (
        id
    ) ON DELETE CASCADE,
    type TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    sort_order INT NOT NULL DEFAULT 0,
    completed BOOL NOT NULL DEFAULT FALSE
);
CREATE INDEX IF NOT EXISTS idx_items_module_id ON learningpaths.items (
    module_id
);

CREATE TABLE IF NOT EXISTS learningpaths.resources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    learning_path_id UUID NOT NULL REFERENCES learningpaths.learning_paths (
        id
    ) ON DELETE CASCADE,
    text TEXT NOT NULL,
    sort_order INT NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_resources_learning_path_id
ON learningpaths.resources (
    learning_path_id
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP SCHEMA IF EXISTS learningpaths CASCADE;
-- +goose StatementEnd
