-- +goose Up
-- +goose StatementBegin
-- Re-key meal plans onto family_id (issue #1380, phase 2 of #1349): every
-- family member gets full read/write on every plan in the family, replacing
-- plan_access's per-plan, per-grant can_edit model entirely.
ALTER TABLE mealplans.plans ADD COLUMN family_id UUID;

-- Backfill: give every distinct plan owner without a family membership yet a
-- family-of-one, mirroring FamilyRepository.EnsureFamily's lazy-creation
-- semantics. gen_random_uuid() in "assigned" is evaluated once per row since
-- Postgres always materializes a WITH query referenced more than once.
WITH missing_users AS (
    SELECT DISTINCT p.owner_user_id AS user_id
    FROM mealplans.plans AS p
    LEFT JOIN global.family_members AS fm ON p.owner_user_id = fm.user_id
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

UPDATE mealplans.plans p
SET family_id = fm.family_id
FROM global.family_members AS fm
WHERE fm.user_id = p.owner_user_id;

ALTER TABLE mealplans.plans ALTER COLUMN family_id SET NOT NULL;
CREATE INDEX idx_plans_family ON mealplans.plans (family_id);

DROP TABLE IF EXISTS mealplans.plan_access;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS mealplans.plan_access (
    plan_id UUID NOT NULL REFERENCES mealplans.plans (id) ON DELETE CASCADE,
    user_id TEXT NOT NULL,
    can_edit BOOL NOT NULL DEFAULT FALSE,
    PRIMARY KEY (plan_id, user_id)
);

DROP INDEX IF EXISTS mealplans.idx_plans_family;
ALTER TABLE mealplans.plans DROP COLUMN IF EXISTS family_id;
-- +goose StatementEnd
