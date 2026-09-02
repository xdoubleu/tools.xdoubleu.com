-- +goose Up
-- +goose StatementBegin
-- Re-key recipes onto family_id (issue #1380, phase 2 of #1349): every family
-- member gets full read/write on the family's one recipe book, replacing
-- recipebook_access's per-grant can_edit model entirely.
ALTER TABLE recipes.recipes ADD COLUMN family_id UUID;

-- Backfill: give every distinct recipe owner without a family membership yet
-- a family-of-one, mirroring FamilyRepository.EnsureFamily's lazy-creation
-- semantics. gen_random_uuid() in "assigned" is evaluated once per row since
-- Postgres always materializes a WITH query referenced more than once.
WITH missing_users AS (
    SELECT DISTINCT r.user_id
    FROM recipes.recipes AS r
    LEFT JOIN global.family_members AS fm ON r.user_id = fm.user_id
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

UPDATE recipes.recipes r
SET family_id = fm.family_id
FROM global.family_members AS fm
WHERE fm.user_id = r.user_id;

ALTER TABLE recipes.recipes ALTER COLUMN family_id SET NOT NULL;
CREATE INDEX idx_recipes_family ON recipes.recipes (family_id);

DROP TABLE IF EXISTS recipes.recipebook_access;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS recipes.recipebook_access (
    owner_user_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    can_edit BOOL NOT NULL DEFAULT TRUE,
    PRIMARY KEY (owner_user_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_recipebook_access_user
ON recipes.recipebook_access (user_id);

DROP INDEX IF EXISTS recipes.idx_recipes_family;
ALTER TABLE recipes.recipes DROP COLUMN IF EXISTS family_id;
-- +goose StatementEnd
