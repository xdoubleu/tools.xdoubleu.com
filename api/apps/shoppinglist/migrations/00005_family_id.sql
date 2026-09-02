-- +goose Up
-- +goose StatementBegin
-- Re-key the shared parts of the shopping list onto family_id (issue #1380,
-- phase 2 of #1349): every family member gets full read/write on the
-- family's one shopping list, replacing shoppinglist_access's per-grant
-- can_edit model entirely. stores/store_categories stay user_id-private per
-- the shoppinglist CLAUDE.md note: a store is tied to one person's shopping
-- route, not the family's shared catalog.
ALTER TABLE shoppinglist.custom_items ADD COLUMN family_id UUID;
ALTER TABLE shoppinglist.categories ADD COLUMN family_id UUID;
ALTER TABLE shoppinglist.item_categories ADD COLUMN family_id UUID;

-- Backfill: give every distinct user across the three tables without a
-- family membership yet a family-of-one, mirroring
-- FamilyRepository.EnsureFamily's lazy-creation semantics. gen_random_uuid()
-- in "assigned" is evaluated once per row since Postgres always materializes
-- a WITH query referenced more than once.
WITH missing_users AS (
    SELECT DISTINCT u.user_id FROM (
        SELECT user_id FROM shoppinglist.custom_items
        UNION
        SELECT user_id FROM shoppinglist.categories
        UNION
        SELECT user_id FROM shoppinglist.item_categories
    ) AS u
    LEFT JOIN global.family_members AS fm ON u.user_id = fm.user_id
    WHERE fm.user_id IS NULL
),

assigned AS (
    SELECT
        user_id,
        gen_random_uuid() AS family_id
    FROM missing_users
),

ins_families AS (
    INSERT INTO global.families (id)
    SELECT family_id FROM assigned
    RETURNING id
)

INSERT INTO global.family_members (user_id, family_id)
SELECT
    user_id,
    family_id
FROM assigned;

UPDATE shoppinglist.custom_items t
SET family_id = fm.family_id
FROM global.family_members AS fm
WHERE fm.user_id = t.user_id;

UPDATE shoppinglist.categories t
SET family_id = fm.family_id
FROM global.family_members AS fm
WHERE fm.user_id = t.user_id;

UPDATE shoppinglist.item_categories t
SET family_id = fm.family_id
FROM global.family_members AS fm
WHERE fm.user_id = t.user_id;

ALTER TABLE shoppinglist.custom_items ALTER COLUMN family_id SET NOT NULL;
ALTER TABLE shoppinglist.categories ALTER COLUMN family_id SET NOT NULL;
ALTER TABLE shoppinglist.item_categories ALTER COLUMN family_id SET NOT NULL;

DROP INDEX IF EXISTS shoppinglist.idx_custom_items_user;
CREATE INDEX idx_custom_items_family ON shoppinglist.custom_items (family_id);
ALTER TABLE shoppinglist.custom_items DROP COLUMN user_id;

DROP INDEX IF EXISTS shoppinglist.idx_categories_user_name;
CREATE UNIQUE INDEX idx_categories_family_name
ON shoppinglist.categories (family_id, lower(name));
ALTER TABLE shoppinglist.categories DROP COLUMN user_id;

ALTER TABLE shoppinglist.item_categories DROP CONSTRAINT item_categories_pkey;
ALTER TABLE shoppinglist.item_categories DROP COLUMN user_id;
ALTER TABLE shoppinglist.item_categories
ADD PRIMARY KEY (family_id, name);

DROP TABLE IF EXISTS shoppinglist.shoppinglist_access;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS shoppinglist.shoppinglist_access (
    owner_user_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    can_edit BOOL NOT NULL DEFAULT TRUE,
    PRIMARY KEY (owner_user_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_shoppinglist_access_user
ON shoppinglist.shoppinglist_access (user_id);

ALTER TABLE shoppinglist.item_categories DROP CONSTRAINT item_categories_pkey;
ALTER TABLE shoppinglist.item_categories
ADD COLUMN user_id TEXT NOT NULL DEFAULT '';
ALTER TABLE shoppinglist.item_categories ADD PRIMARY KEY (user_id, name);

ALTER TABLE shoppinglist.categories ADD COLUMN user_id TEXT NOT NULL DEFAULT '';
DROP INDEX IF EXISTS shoppinglist.idx_categories_family_name;
CREATE UNIQUE INDEX idx_categories_user_name
ON shoppinglist.categories (user_id, lower(name));

ALTER TABLE shoppinglist.custom_items
ADD COLUMN user_id TEXT NOT NULL DEFAULT '';
DROP INDEX IF EXISTS shoppinglist.idx_custom_items_family;
CREATE INDEX idx_custom_items_user ON shoppinglist.custom_items (user_id);

ALTER TABLE shoppinglist.item_categories DROP COLUMN family_id;
ALTER TABLE shoppinglist.categories DROP COLUMN family_id;
ALTER TABLE shoppinglist.custom_items DROP COLUMN family_id;
-- +goose StatementEnd
