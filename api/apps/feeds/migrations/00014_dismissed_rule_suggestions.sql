-- +goose Up
-- +goose StatementBegin
CREATE TABLE feeds.dismissed_rule_suggestions (
    user_id TEXT NOT NULL,
    feed_id UUID NOT NULL REFERENCES feeds.feeds (id) ON DELETE CASCADE,
    category TEXT NOT NULL,
    dismissed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- The key: a primary key can't hold an expression.
CREATE UNIQUE INDEX dismissed_rule_suggestions_key_idx
ON feeds.dismissed_rule_suggestions (user_id, feed_id, lower(category));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE feeds.dismissed_rule_suggestions;
-- +goose StatementEnd
