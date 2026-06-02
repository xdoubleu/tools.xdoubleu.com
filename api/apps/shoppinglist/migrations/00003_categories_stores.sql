-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS shoppinglist.categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id TEXT NOT NULL,
    name TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_categories_user_name
ON shoppinglist.categories (user_id, lower(name));

CREATE TABLE IF NOT EXISTS shoppinglist.item_categories (
    user_id TEXT NOT NULL,
    name TEXT NOT NULL,
    category_id UUID NOT NULL REFERENCES shoppinglist.categories (
        id
    ) ON DELETE CASCADE,
    PRIMARY KEY (user_id, name)
);

CREATE INDEX IF NOT EXISTS idx_item_categories_category
ON shoppinglist.item_categories (category_id);

CREATE TABLE IF NOT EXISTS shoppinglist.stores (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id TEXT NOT NULL,
    name TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_stores_user_name
ON shoppinglist.stores (user_id, lower(name));

CREATE TABLE IF NOT EXISTS shoppinglist.store_categories (
    store_id UUID NOT NULL REFERENCES shoppinglist.stores (
        id
    ) ON DELETE CASCADE,
    category_id UUID NOT NULL REFERENCES shoppinglist.categories (
        id
    ) ON DELETE CASCADE,
    sort_order INT NOT NULL DEFAULT 0,
    PRIMARY KEY (store_id, category_id)
);

CREATE INDEX IF NOT EXISTS idx_store_categories_store
ON shoppinglist.store_categories (store_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS shoppinglist.store_categories;
DROP TABLE IF EXISTS shoppinglist.stores;
DROP TABLE IF EXISTS shoppinglist.item_categories;
DROP TABLE IF EXISTS shoppinglist.categories;
-- +goose StatementEnd
