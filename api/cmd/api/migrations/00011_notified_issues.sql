-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS global.notified_issues (
    key TEXT PRIMARY KEY,
    notified_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS global.notified_issues;
-- +goose StatementEnd
