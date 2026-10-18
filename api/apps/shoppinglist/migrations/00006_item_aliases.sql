-- +goose Up
-- +goose StatementBegin
-- alias is exported as canonical. Both are normalized (lower, trimmed), and a
-- canonical is never itself an alias.
CREATE TABLE IF NOT EXISTS shoppinglist.item_aliases (
    family_id UUID NOT NULL,
    alias TEXT NOT NULL,
    canonical TEXT NOT NULL,
    PRIMARY KEY (family_id, alias),
    CHECK (alias <> canonical)
);

CREATE INDEX IF NOT EXISTS idx_item_aliases_canonical
ON shoppinglist.item_aliases (family_id, canonical);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS shoppinglist.item_aliases;
-- +goose StatementEnd
