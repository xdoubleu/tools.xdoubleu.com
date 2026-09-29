-- +goose Up
-- +goose StatementBegin
CREATE TABLE feeds.filter_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id TEXT NOT NULL,
    -- NULL applies the rule to all of the user's feeds.
    feed_id UUID REFERENCES feeds.feeds (id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('category', 'title')),
    value TEXT NOT NULL CHECK (value <> ''),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- NULLS NOT DISTINCT: global rules are unique too.
CREATE UNIQUE INDEX filter_rules_unique_idx ON feeds.filter_rules (
    user_id, feed_id, kind, lower(value)
) NULLS NOT DISTINCT;

-- New columns, so no backfill: existing items start unfiltered.
ALTER TABLE feeds.items
ADD COLUMN filtered_at TIMESTAMPTZ,
ADD COLUMN filtered_rule_id UUID
REFERENCES feeds.filter_rules (id) ON DELETE SET NULL;

CREATE INDEX items_filtered_rule_id_idx ON feeds.items (filtered_rule_id)
WHERE filtered_rule_id IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE feeds.items
DROP COLUMN filtered_rule_id,
DROP COLUMN filtered_at;
DROP TABLE feeds.filter_rules;
-- +goose StatementEnd
